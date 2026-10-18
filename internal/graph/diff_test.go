package graph

import (
	"testing"
	"time"

	"github.com/pilsuchoi/mcpgraph/internal/model"
)

func server(id, name, app, cmd string) model.Node {
	return model.Node{ID: id, Kind: model.KindMCPServer, Label: name,
		Server: &model.MCPServer{Name: name, App: app, Command: cmd, Transport: "stdio"}}
}

func TestDiff(t *testing.T) {
	host := func(at time.Time) model.Node {
		return model.Node{ID: "h", Kind: model.KindHost, Host: &model.Host{Hostname: "x", ScannedAt: at}}
	}
	old := &model.Graph{Nodes: []model.Node{
		host(time.Unix(100, 0)),
		server("s1", "github", "Cursor", "npx"),
		server("s2", "slack", "Cursor", "npx"),
	}}
	cur := &model.Graph{Nodes: []model.Node{
		host(time.Unix(200, 0)),                 // only scan time differs: not a change
		server("s1", "github", "Cursor", "uvx"), // changed
		server("s3", "notion", "Cursor", "npx"), // added
	}}

	changes := Diff(old, cur)
	want := []struct{ typ, id, msg string }{
		{Changed, "s1", `MCP server "github" in Cursor config changed`},
		{Added, "s3", `new MCP server "notion" added to Cursor config`},
		{Removed, "s2", `MCP server "slack" removed from Cursor config`},
	}
	if len(changes) != len(want) {
		t.Fatalf("changes = %+v", changes)
	}
	for i, w := range want {
		c := changes[i]
		if c.Type != w.typ || c.NodeID != w.id || c.Message != w.msg {
			t.Errorf("change %d = %+v, want %+v", i, c, w)
		}
	}
}

func TestDiffNoChange(t *testing.T) {
	g := &model.Graph{Nodes: []model.Node{server("s1", "github", "Cursor", "npx")}}
	if c := Diff(g, g.Snapshot()); len(c) != 0 {
		t.Fatalf("identical graphs produced changes: %+v", c)
	}
}

func TestDiffFromNil(t *testing.T) {
	g := &model.Graph{Nodes: []model.Node{server("s1", "github", "Cursor", "npx")}}
	if c := Diff(nil, g); len(c) != 1 || c[0].Type != Added {
		t.Fatalf("Diff(nil, g) = %+v", c)
	}
}
