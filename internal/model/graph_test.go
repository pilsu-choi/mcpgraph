package model

import (
	"strings"
	"testing"
)

func TestNodeIDIsStable(t *testing.T) {
	a := NodeID(KindMCPServer, "cursor|/x/mcp.json|github")
	b := NodeID(KindMCPServer, "cursor|/x/mcp.json|github")
	c := NodeID(KindMCPServer, "cursor|/x/mcp.json|slack")

	if a != b {
		t.Fatalf("same key gave different IDs: %s vs %s", a, b)
	}
	if a == c {
		t.Fatalf("different keys gave the same ID: %s", a)
	}
	if !strings.HasPrefix(a, "MCPServer:") || len(a) != len("MCPServer:")+12 {
		t.Fatalf("unexpected ID format: %s", a)
	}
}

func TestSnapshotIsDeepCopy(t *testing.T) {
	g := &Graph{
		Nodes: []Node{{
			ID:     "MCPServer:1",
			Kind:   KindMCPServer,
			Server: &MCPServer{Name: "fs", Args: []string{"/tmp"}},
		}},
		Edges: []Edge{{From: "a", To: "b", Kind: EdgeConfigures}},
	}

	snap := g.Snapshot()
	snap.Nodes[0].Server.Name = "changed"
	snap.Nodes[0].Server.Args[0] = "changed"
	snap.Edges[0].Kind = EdgeRunsAs

	if g.Nodes[0].Server.Name != "fs" || g.Nodes[0].Server.Args[0] != "/tmp" {
		t.Fatalf("snapshot shares node memory with original: %+v", g.Nodes[0].Server)
	}
	if g.Edges[0].Kind != EdgeConfigures {
		t.Fatalf("snapshot shares edge memory with original")
	}
}

func TestOutAndIn(t *testing.T) {
	g := &Graph{
		Nodes: []Node{
			{ID: "app", Kind: KindAIApp},
			{ID: "srv", Kind: KindMCPServer},
		},
		Edges: []Edge{{From: "app", To: "srv", Kind: EdgeConfigures}},
	}
	if out := g.Out("app", EdgeConfigures); len(out) != 1 || out[0].ID != "srv" {
		t.Fatalf("Out = %+v", out)
	}
	if in := g.In("srv", EdgeConfigures); len(in) != 1 || in[0].ID != "app" {
		t.Fatalf("In = %+v", in)
	}
	if out := g.Out("app", EdgeRunsAs); len(out) != 0 {
		t.Fatalf("Out with wrong edge kind = %+v", out)
	}
}
