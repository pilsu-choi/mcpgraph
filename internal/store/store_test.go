package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/graph"
	"github.com/pilsu-choi/mcpgraph/internal/model"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "data", "mcpgraph.db"))
	if err != nil {
		t.Fatal(err)
	}
	// t.Cleanup runs after the test finishes, like a deferred call that
	// outlives this helper function.
	t.Cleanup(func() { s.Close() })
	return s
}

func testGraph(name string) *model.Graph {
	return &model.Graph{Nodes: []model.Node{{
		ID: "MCPServer:" + name, Kind: model.KindMCPServer, Label: name,
		Server: &model.MCPServer{Name: name, App: "Cursor", Transport: "stdio"},
	}}}
}

func TestSaveLatestList(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)

	if g, err := s.Latest(ctx); err != nil || g != nil {
		t.Fatalf("empty store: Latest = %v, %v", g, err)
	}

	findings := []model.Finding{{RuleID: "R1", Severity: model.SeverityHigh}, {RuleID: "R3", Severity: model.SeverityMedium}}
	changes := []graph.Change{{Type: graph.Added, Kind: model.KindMCPServer, NodeID: "MCPServer:a", Message: "new MCP server"}}
	if _, err := s.Save(ctx, time.Unix(100, 0), testGraph("a"), findings, changes); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(ctx, time.Unix(200, 0), testGraph("b"), nil, nil); err != nil {
		t.Fatal(err)
	}

	latest, err := s.Latest(ctx)
	if err != nil || latest.Nodes[0].Label != "b" {
		t.Fatalf("Latest = %+v, %v", latest, err)
	}

	metas, err := s.List(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 2 || metas[0].TakenAt.Unix() != 200 || metas[1].High != 1 || metas[1].Medium != 1 {
		t.Fatalf("List = %+v", metas)
	}
	if len(metas[1].Changes) != 1 || metas[1].Changes[0].Message != "new MCP server" {
		t.Fatalf("changes = %+v", metas[1].Changes)
	}
}

func TestPrune(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	s.keep = 2
	for i := range 5 {
		if _, err := s.Save(ctx, time.Unix(int64(i), 0), testGraph("x"), nil,
			[]graph.Change{{Type: graph.Added, NodeID: "n"}}); err != nil {
			t.Fatal(err)
		}
	}
	metas, err := s.List(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 2 {
		t.Fatalf("kept %d snapshots, want 2", len(metas))
	}
	var orphans int
	if err := s.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM changes WHERE snapshot_id NOT IN (SELECT id FROM snapshots)`).Scan(&orphans); err != nil {
		t.Fatal(err)
	}
	if orphans != 0 {
		t.Fatalf("%d orphan changes left after prune", orphans)
	}
}
