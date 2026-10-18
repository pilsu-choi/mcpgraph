// Package install rewrites AI app configs so that stdio MCP servers run
// through `mcpgraph proxy`, and undoes it.
//
// These are the only commands allowed to write outside mcpgraph's data
// directory (AGENTS.md hard rule 1). Every file is backed up first, the
// change is shown as a diff, and nothing is written without --yes.
// Uninstall restores the backup byte for byte.
package install

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/pilsu-choi/mcpgraph/internal/collector/config"
)

// Options select what install wraps and how.
type Options struct {
	Home string
	GOOS string
	// Bin is the absolute path of the mcpgraph binary written into configs.
	Bin string
	// Apps and Servers filter by app name and server name; empty means all.
	Apps    []string
	Servers []string
	// ProxyFlags are extra flags for `mcpgraph proxy`, e.g. --policy FILE.
	ProxyFlags []string
}

// FileChange is the planned rewrite of one config file.
type FileChange struct {
	App  string
	Path string // as listed in the candidate paths
	Real string // after resolving symlinks; this file is written
	Mode fs.FileMode
	// Before is the file as read; After is the result of Edits.
	Before, After []byte
	Edits         []Edit
	// Servers are the entries changed, as readable locations.
	Servers []string
}

// Skip explains why a server or file was left alone.
type Skip struct {
	App, Path, Server, Reason string
}

// Plan reads every candidate config and works out the edits. It writes
// nothing.
func Plan(opts Options) ([]FileChange, []Skip, error) {
	if !filepath.IsAbs(opts.Bin) {
		return nil, nil, fmt.Errorf("mcpgraph binary path %q is not absolute", opts.Bin)
	}
	var (
		changes []FileChange
		skips   []Skip
		seen    = map[string]bool{}
	)
	for _, cand := range config.Candidates(opts.Home, opts.GOOS) {
		if len(opts.Apps) > 0 && !slices.Contains(opts.Apps, cand.App) {
			continue
		}
		real, data, mode, err := readConfig(cand.Path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			skips = append(skips, Skip{App: cand.App, Path: cand.Path, Reason: err.Error()})
			continue
		}
		if seen[real] {
			continue // two candidate paths pointing at one file
		}
		seen[real] = true

		fc, fileSkips, err := planFile(cand.App, cand.Path, data, opts)
		skips = append(skips, fileSkips...)
		if err != nil {
			skips = append(skips, Skip{App: cand.App, Path: cand.Path, Reason: err.Error()})
			continue
		}
		if len(fc.Edits) == 0 {
			continue
		}
		fc.Real, fc.Mode = real, mode
		changes = append(changes, fc)
	}
	return changes, skips, nil
}

func planFile(app, path string, data []byte, opts Options) (FileChange, []Skip, error) {
	fc := FileChange{App: app, Path: path, Before: data}
	root, err := parseJSON(data)
	if err != nil {
		return fc, nil, fmt.Errorf("parse: %w", err)
	}
	var skips []Skip
	skip := func(e entry, reason string) {
		skips = append(skips, Skip{App: app, Path: path, Server: e.where, Reason: reason})
	}
	for _, e := range findEntries(app, root, data) {
		if len(opts.Servers) > 0 && !slices.Contains(opts.Servers, e.name) {
			continue
		}
		switch {
		case !e.stdio:
			skip(e, "not a stdio server; the proxy wraps stdio only")
		case e.proxied():
			skip(e, "already runs through mcpgraph proxy")
		case e.args != nil && e.args.kind != 'a':
			skip(e, `"args" is not an array`)
		default:
			args := append([]string{"proxy", "--server", e.name, "--app", app}, opts.ProxyFlags...)
			args = append(args, "--", e.cmd)
			args = append(args, e.argv...)
			fc.Edits = append(fc.Edits, rewrite(e, opts.Bin, args)...)
			fc.Servers = append(fc.Servers, e.where)
		}
	}
	fc.After = applyEdits(data, fc.Edits)
	// Belt and braces: the result must still be a config the app can read.
	if len(fc.Edits) > 0 {
		if _, err := parseJSON(fc.After); err != nil {
			return fc, skips, fmt.Errorf("rewritten file would not parse (bug): %w", err)
		}
	}
	return fc, skips, nil
}

// readConfig resolves symlinks and reads a config file with a size limit.
func readConfig(path string) (real string, data []byte, mode fs.FileMode, err error) {
	real, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", nil, 0, err
	}
	info, err := os.Stat(real)
	if err != nil {
		return "", nil, 0, err
	}
	if !info.Mode().IsRegular() {
		return "", nil, 0, fmt.Errorf("%s is not a regular file", real)
	}
	if info.Size() > config.MaxConfigSize {
		return "", nil, 0, fmt.Errorf("file is %d bytes, larger than limit %d", info.Size(), config.MaxConfigSize)
	}
	data, err = os.ReadFile(real)
	return real, data, info.Mode().Perm(), err
}

func sha(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
