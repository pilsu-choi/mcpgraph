package rules

import (
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

func proc(id string, pid int32, mcp bool) model.Node {
	return model.Node{ID: id, Kind: model.KindProcess, Process: &model.Process{PID: pid, MCPLike: mcp, Cmdline: "npm exec @playwright/mcp@latest"}}
}

func TestShadowServer(t *testing.T) {
	tests := []struct {
		name  string
		build func(b *graphBuilder)
		want  int
	}{
		{
			name: "undeclared chain reports only the top process",
			build: func(b *graphBuilder) {
				b.node(proc("p1", 1, true)).node(proc("p2", 2, true)).
					edge("p2", "p1", model.EdgeChildOf)
			},
			want: 1,
		},
		{
			name: "declared server and its children",
			build: func(b *graphBuilder) {
				b.server("s", model.MCPServer{Name: "pw"}).
					node(proc("p1", 1, true)).node(proc("p2", 2, true)).
					edge("s", "p1", model.EdgeRunsAs).
					edge("p2", "p1", model.EdgeChildOf)
			},
			want: 0,
		},
		{
			name: "process that is not MCP-like",
			build: func(b *graphBuilder) {
				b.node(proc("p1", 1, false))
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &graphBuilder{}
			tt.build(b)
			checkFindings(t, ShadowServer{}, &b.g, tt.want, model.SeverityMedium)
		})
	}
}
