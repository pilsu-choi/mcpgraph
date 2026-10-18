package graph

import (
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

func TestMatches(t *testing.T) {
	tests := []struct {
		name   string
		server model.MCPServer
		proc   model.Process
		want   bool
	}{
		{
			name:   "npx package",
			server: model.MCPServer{Transport: "stdio", Command: "npx", Args: []string{"-y", "@playwright/mcp@latest"}},
			proc:   model.Process{Args: []string{"npm", "exec", "@playwright/mcp@latest"}},
			want:   true,
		},
		{
			name:   "absolute command path in process",
			server: model.MCPServer{Transport: "stdio", Command: "github-mcp-server"},
			proc:   model.Process{Args: []string{"/usr/local/bin/github-mcp-server"}},
			want:   true,
		},
		{
			name:   "masked secret compares equal",
			server: model.MCPServer{Transport: "stdio", Command: "node", Args: []string{"/opt/s.js", "--api-key", "****FAKE"}},
			proc:   model.Process{Args: []string{"node", "/opt/s.js", "--api-key", "****FAKE"}},
			want:   true,
		},
		{
			name:   "different root dir",
			server: model.MCPServer{Transport: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem", "/a"}},
			proc:   model.Process{Args: []string{"npm", "exec", "@modelcontextprotocol/server-filesystem", "/b"}},
			want:   false,
		},
		{
			name:   "http server never matches a process",
			server: model.MCPServer{Transport: "http", URL: "http://localhost:3000"},
			proc:   model.Process{Args: []string{"node", "server.js"}},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Matches(&tt.server, &tt.proc); got != tt.want {
				t.Errorf("Matches = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLinkPrefersOwningApp(t *testing.T) {
	args := []string{"-y", "@playwright/mcp@latest"}
	g := &model.Graph{
		Nodes: []model.Node{
			{ID: "s-cursor", Kind: model.KindMCPServer, Server: &model.MCPServer{App: "Cursor", Transport: "stdio", Command: "npx", Args: args}},
			{ID: "s-claude", Kind: model.KindMCPServer, Server: &model.MCPServer{App: "Claude Code", Transport: "stdio", Command: "npx", Args: args}},
			{ID: "p-app", Kind: model.KindProcess, Process: &model.Process{PID: 1, AIApp: "Claude Code"}},
			{ID: "p-srv", Kind: model.KindProcess, Process: &model.Process{PID: 2, Args: []string{"npm", "exec", "@playwright/mcp@latest"}}},
		},
		Edges: []model.Edge{{From: "p-srv", To: "p-app", Kind: model.EdgeChildOf}},
	}
	linkProcesses(g)

	var runsAs []model.Edge
	for _, e := range g.Edges {
		if e.Kind == model.EdgeRunsAs {
			runsAs = append(runsAs, e)
		}
	}
	if len(runsAs) != 1 || runsAs[0].From != "s-claude" || runsAs[0].To != "p-srv" {
		t.Fatalf("RUNS_AS edges = %+v, want only s-claude -> p-srv", runsAs)
	}
}
