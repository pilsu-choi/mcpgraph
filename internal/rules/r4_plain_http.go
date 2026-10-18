package rules

import (
	"fmt"
	"net/url"

	"github.com/pilsuchoi/mcpgraph/internal/model"
	"github.com/pilsuchoi/mcpgraph/internal/netutil"
)

// PlainHTTP (R4) flags remote MCP servers reached over unencrypted HTTP.
type PlainHTTP struct{}

// ID implements Rule.
func (PlainHTTP) ID() string { return "R4" }

// Eval implements Rule.
func (r PlainHTTP) Eval(g *model.Graph) []model.Finding {
	var out []model.Finding
	for _, s := range g.NodesOfKind(model.KindMCPServer) {
		u, err := url.Parse(s.Server.URL)
		if err != nil || u.Scheme != "http" || netutil.IsLoopbackHost(u.Hostname()) {
			continue
		}
		out = append(out, model.Finding{
			RuleID:   r.ID(),
			Severity: model.SeverityMedium,
			NodeID:   s.ID,
			Title:    fmt.Sprintf("%s uses plain http:// to %s", serverName(s), u.Host),
			Message: "Tool calls, results and any auth headers cross the network unencrypted and can be read " +
				"or modified in transit; use https://.",
		})
	}
	return out
}
