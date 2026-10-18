// Package rules evaluates risk rules against a graph.
//
// Each rule lives in its own file and is a small struct that satisfies the
// Rule interface. Rules only read the graph; they never modify it.
package rules

import (
	"sort"

	"github.com/pilsuchoi/mcpgraph/internal/model"
)

// Rule inspects a graph and returns findings.
type Rule interface {
	ID() string
	Eval(g *model.Graph) []model.Finding
}

// Default returns every MVP rule. home is the scanned user's home
// directory, which R2 and R5 need.
func Default(home string) []Rule {
	return []Rule{
		PlaintextSecret{},
		BroadFilesystem{Home: home},
		UnpinnedRunner{},
		PlainHTTP{},
		CredentialFiles{Home: home},
		ShadowServer{},
	}
}

// Evaluate runs every rule and returns all findings, most severe first.
func Evaluate(g *model.Graph, rules []Rule) []model.Finding {
	var out []model.Finding
	for _, r := range rules {
		out = append(out, r.Eval(g)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if rank(a.Severity) != rank(b.Severity) {
			return rank(a.Severity) < rank(b.Severity)
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		return a.Title < b.Title
	})
	return out
}

// HasHigh reports whether any finding is high severity.
func HasHigh(findings []model.Finding) bool {
	for _, f := range findings {
		if f.Severity == model.SeverityHigh {
			return true
		}
	}
	return false
}

func rank(s model.Severity) int {
	switch s {
	case model.SeverityHigh:
		return 0
	case model.SeverityMedium:
		return 1
	default:
		return 2
	}
}

// serverName formats "App / name" for finding titles.
func serverName(n model.Node) string {
	return n.Server.App + " / " + n.Server.Name
}
