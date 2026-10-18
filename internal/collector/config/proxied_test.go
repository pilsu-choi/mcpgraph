package config

import (
	"slices"
	"testing"
)

func TestUnwrapProxy(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		args        []string
		wantProxied bool
		wantCommand string
		wantArgs    []string
	}{
		{"plain server", "npx", []string{"-y", "pkg"}, false, "npx", []string{"-y", "pkg"}},
		{"proxied", "/usr/local/bin/mcpgraph", []string{"proxy", "--", "npx", "-y", "pkg"}, true, "npx", []string{"-y", "pkg"}},
		{"proxied with flags", "mcpgraph", []string{"proxy", "--name", "gh", "--", "uvx", "srv"}, true, "uvx", []string{"srv"}},
		{"proxied no args", "mcpgraph", []string{"proxy", "--", "server-bin"}, true, "server-bin", []string{}},
		{"missing separator", "mcpgraph", []string{"proxy", "npx"}, false, "mcpgraph", []string{"proxy", "npx"}},
		{"nothing after separator", "mcpgraph", []string{"proxy", "--"}, false, "mcpgraph", []string{"proxy", "--"}},
		{"other subcommand", "mcpgraph", []string{"scan"}, false, "mcpgraph", []string{"scan"}},
		{"other binary named proxy", "node", []string{"proxy", "--", "x"}, false, "node", []string{"proxy", "--", "x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, proxied := unwrapProxy(serverConfig{Command: tt.command, Args: tt.args})
			if proxied != tt.wantProxied || got.Command != tt.wantCommand || !slices.Equal(got.Args, tt.wantArgs) {
				t.Errorf("got (%q %q %v), want (%q %q %v)", got.Command, got.Args, proxied, tt.wantCommand, tt.wantArgs, tt.wantProxied)
			}
		})
	}
}
