package config

import (
	"os"
	"reflect"
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/testutil"
)

func TestParsers(t *testing.T) {
	tests := []struct {
		name      string
		parser    parser
		fixture   string
		wantNames []string
		wantPaths []string
	}{
		{
			name:      "claude desktop",
			parser:    claudeDesktop{},
			fixture:   "claude_desktop/claude_desktop_config.json",
			wantNames: []string{"filesystem", "github", "postgres"},
			wantPaths: []string{"$.mcpServers.filesystem", "$.mcpServers.github", "$.mcpServers.postgres"},
		},
		{
			name:      "claude code user and project scope",
			parser:    claudeCode{},
			fixture:   "claude_code/.claude.json",
			wantNames: []string{"sentry", "slack"},
			wantPaths: []string{"$.mcpServers.sentry", `$.projects["/work/app"].mcpServers.slack`},
		},
		{
			name:      "cursor",
			parser:    cursor{},
			fixture:   "cursor/mcp.json",
			wantNames: []string{"git", "local-http", "remote-insecure"},
			wantPaths: []string{"$.mcpServers.git", "$.mcpServers.local-http", "$.mcpServers.remote-insecure"},
		},
		{
			name:      "vscode jsonc",
			parser:    vscode{},
			fixture:   "vscode/mcp.json",
			wantNames: []string{"custom", "github-remote"},
			wantPaths: []string{"$.servers.custom", "$.servers.github-remote"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(testutil.Fixture(t, tt.fixture))
			if err != nil {
				t.Fatal(err)
			}
			servers, err := tt.parser.Parse(data)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			var names, paths []string
			for _, s := range servers {
				names = append(names, s.Name)
				paths = append(paths, s.JSONPath)
			}
			if !reflect.DeepEqual(names, tt.wantNames) {
				t.Errorf("names = %v, want %v", names, tt.wantNames)
			}
			if !reflect.DeepEqual(paths, tt.wantPaths) {
				t.Errorf("paths = %v, want %v", paths, tt.wantPaths)
			}
		})
	}
}

func TestParseFieldDetails(t *testing.T) {
	data, err := os.ReadFile(testutil.Fixture(t, "vscode/mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	servers, err := vscode{}.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	custom := servers[0]
	if custom.Command != "node" || len(custom.Args) != 3 || custom.EnvFile != "${workspaceFolder}/.env" {
		t.Errorf("custom = %+v", custom)
	}
	remote := servers[1]
	if remote.Type != "http" || remote.URL != "https://api.githubcopilot.com/mcp/" || remote.Headers["Authorization"] == "" {
		t.Errorf("github-remote = %+v", remote)
	}
}

func TestParseInvalidJSON(t *testing.T) {
	for _, p := range []parser{claudeDesktop{}, claudeCode{}, cursor{}, vscode{}} {
		if _, err := p.Parse([]byte(`{"mcpServers": `)); err == nil {
			t.Errorf("%s: expected error for truncated JSON", p.App())
		}
	}
}

func TestStripJSONC(t *testing.T) {
	tests := []struct{ in, want string }{
		{`{"a": 1, // comment` + "\n" + `}`, "{\"a\": 1 \n}"},
		{`{"u": "http://x"}`, `{"u": "http://x"}`},
		{`{/* c */"a": [1, 2,]}`, `{"a": [1, 2]}`},
		{`{"s": "a // not a comment", }`, `{"s": "a // not a comment" }`},
		{`{"q": "escaped \" // still string"}`, `{"q": "escaped \" // still string"}`},
	}
	for _, tt := range tests {
		if got := string(stripJSONC([]byte(tt.in))); got != tt.want {
			t.Errorf("stripJSONC(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
