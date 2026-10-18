package rules

import (
	"testing"

	"github.com/pilsuchoi/mcpgraph/internal/model"
)

func TestPlaintextSecret(t *testing.T) {
	cred := func(kind string) *model.Graph {
		b := &graphBuilder{}
		b.server("s", model.MCPServer{Name: "github"}).
			node(model.Node{ID: "c", Kind: model.KindCredential, Credential: &model.Credential{
				Kind: kind, KeyName: "GITHUB_TOKEN", Location: "/x/mcp.json#$.mcpServers.github.env.GITHUB_TOKEN", Reason: "known token prefix ghp_"}}).
			edge("s", "c", model.EdgeUsesSecret)
		return &b.g
	}
	tests := []struct {
		name string
		g    *model.Graph
		want int
	}{
		{"inline secret", cred(model.CredentialInline), 1},
		{"env reference", cred(model.CredentialEnv), 0},
		{"credential file path", cred(model.CredentialFile), 0},
		{"no credentials", &model.Graph{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkFindings(t, PlaintextSecret{}, tt.g, tt.want, model.SeverityHigh)
		})
	}
}
