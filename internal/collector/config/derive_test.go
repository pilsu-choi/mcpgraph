package config

import (
	"path/filepath"
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/testutil"
)

// nodesByLabel scans one fixture and indexes the resulting nodes by
// kind and label.
func nodesByLabel(t *testing.T, home, app string) map[string]model.Node {
	t.Helper()
	out := map[string]model.Node{}
	for _, c := range Candidates(home, "linux") {
		if c.App != app {
			continue
		}
		ev := ScanCandidate(c, home)
		for _, n := range ev.Nodes {
			out[string(n.Kind)+"/"+n.Label] = n
		}
	}
	return out
}

func TestDeriveCredentialsAndDataSources(t *testing.T) {
	home := testutil.FakeHome(t)

	tests := []struct {
		app      string
		key      string // kind/label
		credKind string // for Credential nodes
		dsKind   string // for DataSource nodes
	}{
		{"Claude Desktop", "DataSource/" + home, "", model.DataSourceFilesystem},
		{"Claude Desktop", "DataSource/github", "", model.DataSourceSaaS},
		{"Claude Desktop", "DataSource/postgresql://db.internal:5432/prod", "", model.DataSourceDatabase},
		{"Claude Desktop", "Credential/GITHUB_PERSONAL_ACCESS_TOKEN", model.CredentialInline, ""},
		{"Claude Desktop", "Credential/args[2]", model.CredentialInline, ""},
		{"Claude Code", "Credential/SLACK_BOT_TOKEN", model.CredentialInline, ""},
		{"Claude Code", "DataSource/mcp.sentry.dev", "", model.DataSourceSaaS},
		{"Cursor", "Credential/GIT_TOKEN", model.CredentialEnv, ""},
		{"Cursor", "Credential/url", model.CredentialInline, ""},
		{"Cursor", "DataSource/" + filepath.Join(home, "code/project"), "", model.DataSourceFilesystem},
		{"VS Code", "Credential/--api-key", model.CredentialInline, ""},
		{"VS Code", "Credential/API_KEY", model.CredentialInline, ""},
		{"VS Code", "Credential/envFile", model.CredentialFile, ""},
		{"VS Code", "Credential/Authorization", model.CredentialEnv, ""},
	}
	cache := map[string]map[string]model.Node{}
	for _, tt := range tests {
		t.Run(tt.app+" "+tt.key, func(t *testing.T) {
			if cache[tt.app] == nil {
				cache[tt.app] = nodesByLabel(t, home, tt.app)
			}
			n, ok := cache[tt.app][tt.key]
			if !ok {
				t.Fatalf("node %s not found; have %v", tt.key, keys(cache[tt.app]))
			}
			if tt.credKind != "" && n.Credential.Kind != tt.credKind {
				t.Errorf("credential kind = %s, want %s", n.Credential.Kind, tt.credKind)
			}
			if tt.dsKind != "" && n.DataSource.Kind != tt.dsKind {
				t.Errorf("data source kind = %s, want %s", n.DataSource.Kind, tt.dsKind)
			}
		})
	}
}

func TestDeriveNoFalseCredentials(t *testing.T) {
	home := testutil.FakeHome(t)
	nodes := nodesByLabel(t, home, "Claude Code")
	// SLACK_TEAM_ID is an identifier, not a secret.
	if _, ok := nodes["Credential/SLACK_TEAM_ID"]; ok {
		t.Error("SLACK_TEAM_ID should not be a credential")
	}
}

func TestTransport(t *testing.T) {
	tests := []struct {
		in   serverConfig
		want string
	}{
		{serverConfig{Command: "npx"}, model.TransportStdio},
		{serverConfig{URL: "https://x"}, model.TransportHTTP},
		{serverConfig{Type: "sse", URL: "https://x"}, model.TransportSSE},
		{serverConfig{Type: "streamable-http"}, model.TransportHTTP},
	}
	for _, tt := range tests {
		if got := transport(tt.in); got != tt.want {
			t.Errorf("transport(%+v) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func keys(m map[string]model.Node) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
