package posture

import (
	"fmt"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// ShadowServer (R6) flags running processes that look like MCP servers but
// are not declared in any config we know about.
type ShadowServer struct{}

// ID implements Rule.
func (ShadowServer) ID() string { return "R6" }

// Eval implements Rule.
func (r ShadowServer) Eval(g *model.Graph) []model.Finding {
	var out []model.Finding
	for _, p := range g.NodesOfKind(model.KindProcess) {
		if !p.Process.MCPLike || declared(g, p.ID) || parentIsMCPLike(g, p.ID) {
			continue
		}
		out = append(out, model.Finding{
			RuleID:   r.ID(),
			Severity: model.SeverityMedium,
			NodeID:   p.ID,
			Title:    fmt.Sprintf("Undeclared MCP-like process pid %d", p.Process.PID),
			Message: fmt.Sprintf("%q is not in any scanned config, so its tools and permissions are invisible to "+
				"review; declare it or find out what started it.", p.Process.Cmdline),
		})
	}
	return out
}

// declared reports whether the process, or any ancestor, is linked to a
// declared server by a RUNS_AS edge. Children of a declared server (e.g.
// the node process that npx starts) count as declared too.
func declared(g *model.Graph, id string) bool {
	seen := map[string]bool{}
	for id != "" && !seen[id] {
		seen[id] = true
		if len(g.In(id, model.EdgeRunsAs)) > 0 {
			return true
		}
		id = parentID(g, id)
	}
	return false
}

// parentIsMCPLike avoids reporting every process in a chain such as
// "npm exec x" -> "sh -c x" -> "node x": only the topmost is reported.
func parentIsMCPLike(g *model.Graph, id string) bool {
	parents := g.Out(id, model.EdgeChildOf)
	return len(parents) > 0 && parents[0].Process.MCPLike
}

func parentID(g *model.Graph, id string) string {
	parents := g.Out(id, model.EdgeChildOf)
	if len(parents) == 0 {
		return ""
	}
	return parents[0].ID
}
