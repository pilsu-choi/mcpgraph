package posture

import (
	"fmt"
	"path/filepath"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// BroadFilesystem (R2) flags MCP servers given the whole home directory or
// the whole filesystem.
type BroadFilesystem struct {
	Home string
}

// ID implements Rule.
func (BroadFilesystem) ID() string { return "R2" }

// Eval implements Rule.
func (r BroadFilesystem) Eval(g *model.Graph) []model.Finding {
	home := filepath.Clean(r.Home)
	var out []model.Finding
	for _, s := range g.NodesOfKind(model.KindMCPServer) {
		for _, ds := range g.Out(s.ID, model.EdgeAccesses) {
			if ds.DataSource.Kind != model.DataSourceFilesystem {
				continue
			}
			root := filepath.Clean(ds.DataSource.Identifier)
			if root != "/" && root != home {
				continue
			}
			out = append(out, model.Finding{
				RuleID:   r.ID(),
				Severity: model.SeverityHigh,
				NodeID:   s.ID,
				Title:    fmt.Sprintf("%s can access %s", serverName(s), root),
				Message: "The server is rooted at the home directory or /, so a prompt injection can read SSH keys, " +
					"cloud credentials and browser data; scope it to the project directory.",
			})
		}
	}
	return out
}
