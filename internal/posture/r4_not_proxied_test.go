package posture

import (
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

func TestNotProxied(t *testing.T) {
	tests := []struct {
		name   string
		server model.MCPServer
		want   int
	}{
		{"direct stdio", model.MCPServer{Name: "gh", Command: "npx"}, 1},
		{"proxied stdio", model.MCPServer{Name: "gh", Command: "npx", Proxied: true}, 0},
		{"remote http", model.MCPServer{Name: "r", Transport: model.TransportHTTP, URL: "https://x/mcp"}, 0},
		{"remote sse", model.MCPServer{Name: "r", Transport: model.TransportSSE, URL: "https://x/sse"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &graphBuilder{}
			b.server("s", tt.server)
			checkFindings(t, NotProxied{}, &b.g, tt.want, model.SeverityMedium)
		})
	}
}
