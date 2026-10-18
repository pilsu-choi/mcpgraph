package posture

import (
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

func TestEvaluateSortsBySeverity(t *testing.T) {
	b := &graphBuilder{}
	b.server("s1", model.MCPServer{Name: "a", Command: "npx", Args: []string{"pkg"}}). // R3 medium
												server("s2", model.MCPServer{Name: "b", Command: "npx", Args: []string{"@x/server-filesystem@1.0.0", "/home/u"}}).
												node(model.Node{ID: "ds", Kind: model.KindDataSource, DataSource: &model.DataSource{Kind: model.DataSourceFilesystem, Identifier: "/home/u"}}).
												edge("s2", "ds", model.EdgeAccesses) // R2 high

	got := Evaluate(&b.g, []Rule{UnpinnedRunner{}, BroadFilesystem{Home: "/home/u"}})
	if len(got) != 2 || got[0].Severity != model.SeverityHigh || got[1].Severity != model.SeverityMedium {
		t.Fatalf("findings not sorted by severity: %+v", got)
	}
	if !HasHigh(got) || HasHigh(got[1:]) {
		t.Error("HasHigh wrong")
	}
}

func TestDefaultRuleIDsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Default("/home/u") {
		if seen[r.ID()] {
			t.Errorf("duplicate rule ID %s", r.ID())
		}
		seen[r.ID()] = true
	}
	if len(seen) != 7 {
		t.Errorf("want 7 rules, got %d", len(seen))
	}
}
