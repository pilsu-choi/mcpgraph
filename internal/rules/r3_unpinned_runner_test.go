package rules

import (
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

func TestUnpinnedRunner(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		args []string
		want int
	}{
		{"npx unpinned", "npx", []string{"-y", "@modelcontextprotocol/server-github"}, 1},
		{"npx latest tag", "npx", []string{"-y", "@playwright/mcp@latest"}, 1},
		{"npx caret range", "npx", []string{"pkg@^1.2.0"}, 1},
		{"npx x range", "npx", []string{"pkg@1.x"}, 1},
		{"npx full path", "/usr/local/bin/npx", []string{"-y", "pkg"}, 1},
		{"uvx unpinned", "uvx", []string{"mcp-server-git", "--repository", "/r"}, 1},
		{"uvx --from unpinned", "uvx", []string{"--from", "mcp-server-git", "mcp-server-git"}, 1},
		{"npx pinned scoped", "npx", []string{"-y", "@modelcontextprotocol/server-postgres@0.6.2"}, 0},
		{"npx pinned", "npx", []string{"slack-mcp-server@1.2.0"}, 0},
		{"npx --package pinned", "npx", []string{"--package=pkg@2.0.0", "bin"}, 0},
		{"uvx pinned ==", "uvx", []string{"mcp-server-git==0.6.2"}, 0},
		{"uvx pinned @", "uvx", []string{"mcp-server-git@0.6.2"}, 0},
		{"not a runner", "node", []string{"/opt/server.js"}, 0},
		{"remote server", "", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &graphBuilder{}
			b.server("s", model.MCPServer{Name: "x", Command: tt.cmd, Args: tt.args})
			checkFindings(t, UnpinnedRunner{}, &b.g, tt.want, model.SeverityMedium)
		})
	}
}
