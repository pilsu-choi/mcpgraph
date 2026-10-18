package posture

import (
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// graphBuilder makes small graphs for rule tests.
type graphBuilder struct{ g model.Graph }

func (b *graphBuilder) server(id string, s model.MCPServer) *graphBuilder {
	if s.App == "" {
		s.App = "Cursor"
	}
	if s.Transport == "" {
		s.Transport = model.TransportStdio
	}
	b.g.Nodes = append(b.g.Nodes, model.Node{ID: id, Kind: model.KindMCPServer, Label: s.Name, Server: &s})
	return b
}

func (b *graphBuilder) node(n model.Node) *graphBuilder {
	b.g.Nodes = append(b.g.Nodes, n)
	return b
}

func (b *graphBuilder) edge(from, to string, kind model.EdgeKind) *graphBuilder {
	b.g.Edges = append(b.g.Edges, model.Edge{From: from, To: to, Kind: kind})
	return b
}

// fsServer is a server with ACCESSES to one filesystem root.
func fsServer(root string) *model.Graph {
	b := &graphBuilder{}
	b.server("s", model.MCPServer{Name: "fs"}).
		node(model.Node{ID: "ds", Kind: model.KindDataSource, DataSource: &model.DataSource{Kind: model.DataSourceFilesystem, Identifier: root}}).
		edge("s", "ds", model.EdgeAccesses)
	return &b.g
}

// checkFindings asserts the number of findings, their rule ID, severity
// and that every finding carries a rationale.
func checkFindings(t *testing.T, r Rule, g *model.Graph, want int, sev model.Severity) []model.Finding {
	t.Helper()
	got := r.Eval(g)
	if len(got) != want {
		t.Fatalf("%s: %d findings, want %d: %+v", r.ID(), len(got), want, got)
	}
	for _, f := range got {
		if f.RuleID != r.ID() || f.Severity != sev || f.Message == "" || f.Title == "" || f.NodeID == "" {
			t.Errorf("malformed finding: %+v", f)
		}
	}
	return got
}
