package posture

import (
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

func TestPlainHTTP(t *testing.T) {
	tests := []struct {
		url  string
		want int
	}{
		{"http://10.0.0.5:8080/mcp", 1},
		{"http://mcp.example.com/mcp?api_key=****abcd", 1},
		{"https://mcp.example.com/mcp", 0},
		{"http://localhost:3000/mcp", 0},
		{"http://127.0.0.1:3000/mcp", 0},
		{"http://[::1]:3000/mcp", 0},
		{"", 0},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			b := &graphBuilder{}
			b.server("s", model.MCPServer{Name: "remote", Transport: model.TransportHTTP, URL: tt.url})
			checkFindings(t, PlainHTTP{}, &b.g, tt.want, model.SeverityMedium)
		})
	}
}
