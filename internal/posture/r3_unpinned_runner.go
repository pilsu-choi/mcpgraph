package posture

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// UnpinnedRunner (R3) flags servers launched through a package runner
// (npx, uvx, ...) without a fixed version. Every start may then download
// and run whatever the registry currently serves.
type UnpinnedRunner struct{}

// ID implements Rule.
func (UnpinnedRunner) ID() string { return "R3" }

// Eval implements Rule.
func (r UnpinnedRunner) Eval(g *model.Graph) []model.Finding {
	var out []model.Finding
	for _, s := range g.NodesOfKind(model.KindMCPServer) {
		runner := filepath.Base(s.Server.Command)
		pkg, pinned, ok := runnerPackage(runner, s.Server.Args)
		if !ok || pinned {
			continue
		}
		out = append(out, model.Finding{
			RuleID:   r.ID(),
			Severity: model.SeverityMedium,
			NodeID:   s.ID,
			Title:    fmt.Sprintf("%s runs unpinned package %s via %s", serverName(s), pkg, runner),
			Message: "Without a version, every start fetches the latest release, so a compromised or typosquatted " +
				"publish runs immediately; pin an exact version.",
		})
	}
	return out
}

// runnerPackage finds the package a runner command starts and whether it
// has a version. ok is false if the command is not a known runner.
func runnerPackage(runner string, args []string) (pkg string, pinned, ok bool) {
	switch runner {
	case "npx", "bunx", "pnpx":
		pkg = firstPackage(args, []string{"-p", "--package"})
		if pkg == "" {
			return "", false, false
		}
		return pkg, npmPinned(pkg), true
	case "uvx":
		pkg = firstPackage(args, []string{"--from"})
		if pkg == "" {
			return "", false, false
		}
		return pkg, pyPinned(pkg), true
	}
	return "", false, false
}

// firstPackage returns the value of a package flag if present, otherwise
// the first argument that is not a flag.
func firstPackage(args, pkgFlags []string) string {
	for i, a := range args {
		for _, f := range pkgFlags {
			if a == f && i+1 < len(args) {
				return args[i+1]
			}
			if strings.HasPrefix(a, f+"=") {
				return strings.TrimPrefix(a, f+"=")
			}
		}
	}
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}

// npmPinned reports whether an npm package spec has an exact version:
// "pkg@1.2.3" and "@scope/pkg@1.2.3" are pinned; "pkg", "@scope/pkg",
// "pkg@latest", "pkg@^1.2" and "pkg@1.x" are not. Dist-tags such as
// "latest" can be moved by the publisher, so only versions that start with
// a digit and contain no range characters count.
func npmPinned(spec string) bool {
	// LastIndex skips the leading "@" of a scoped package; at <= 0 means
	// there is no version separator at all.
	at := strings.LastIndex(spec, "@")
	if at <= 0 {
		return false
	}
	version := spec[at+1:]
	if version == "" || version[0] < '0' || version[0] > '9' {
		return false
	}
	return !strings.ContainsAny(version, "^~*<>| x")
}

// pyPinned reports whether a Python package spec has an exact version:
// "pkg==1.2" or "pkg@1.2".
func pyPinned(spec string) bool {
	if strings.Contains(spec, "==") {
		return true
	}
	at := strings.LastIndex(spec, "@")
	return at > 0 && spec[at+1:] != "" && spec[at+1:] != "latest"
}
