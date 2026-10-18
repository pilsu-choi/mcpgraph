package rules

import (
	"fmt"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// PlaintextSecret (R1) flags secrets written literally into an MCP
// server's env, args, headers or URL.
type PlaintextSecret struct{}

// ID implements Rule.
func (PlaintextSecret) ID() string { return "R1" }

// Eval implements Rule.
func (r PlaintextSecret) Eval(g *model.Graph) []model.Finding {
	var out []model.Finding
	for _, s := range g.NodesOfKind(model.KindMCPServer) {
		for _, c := range g.Out(s.ID, model.EdgeUsesSecret) {
			if c.Credential.Kind != model.CredentialInline {
				continue
			}
			out = append(out, model.Finding{
				RuleID:   r.ID(),
				Severity: model.SeverityHigh,
				NodeID:   c.ID,
				Title:    fmt.Sprintf("Plaintext secret %s in %s", c.Credential.KeyName, serverName(s)),
				Message: fmt.Sprintf("%s (%s). Config files are synced, backed up and read by other tools; "+
					"reference an environment variable or secret store instead.", c.Credential.Location, c.Credential.Reason),
			})
		}
	}
	return out
}
