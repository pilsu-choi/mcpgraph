package posture

import (
	"fmt"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// NotProxied (R4) flags stdio MCP servers that run directly instead of
// through `mcpgraph proxy`. Their tool calls bypass the policy engine.
//
// Remote (http/sse) servers are skipped: the proxy handles stdio only, so
// flagging them would be a finding nobody can fix.
type NotProxied struct{}

// ID implements Rule.
func (NotProxied) ID() string { return "R4" }

// Eval implements Rule.
func (r NotProxied) Eval(g *model.Graph) []model.Finding {
	var out []model.Finding
	for _, s := range g.NodesOfKind(model.KindMCPServer) {
		if s.Server.Transport != model.TransportStdio || s.Server.Proxied {
			continue
		}
		out = append(out, model.Finding{
			RuleID:   r.ID(),
			Severity: model.SeverityMedium,
			NodeID:   s.ID,
			Title:    fmt.Sprintf("%s is not routed through the mcpgraph proxy", serverName(s)),
			Message: "Its tool calls reach the server without a policy check or audit record; " +
				"run `mcpgraph install` to wrap it.",
		})
	}
	return out
}
