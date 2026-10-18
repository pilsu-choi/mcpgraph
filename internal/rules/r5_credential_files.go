package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// CredentialFiles (R5) flags well-known credential files that sit inside a
// directory an MCP server can access.
//
// It checks only whether the files exist (stat/glob); it never opens them.
type CredentialFiles struct {
	Home string
	// Exists and Glob default to the real filesystem. Tests can replace
	// them; a nil func field means "use the default".
	Exists func(path string) bool
	Glob   func(pattern string) []string
}

// ID implements Rule.
func (CredentialFiles) ID() string { return "R5" }

// Eval implements Rule.
func (r CredentialFiles) Eval(g *model.Graph) []model.Finding {
	exists, glob := r.Exists, r.Glob
	if exists == nil {
		exists = func(p string) bool { _, err := os.Stat(p); return err == nil }
	}
	if glob == nil {
		glob = func(p string) []string { m, _ := filepath.Glob(p); return m }
	}

	var out []model.Finding
	for _, s := range g.NodesOfKind(model.KindMCPServer) {
		for _, ds := range g.Out(s.ID, model.EdgeAccesses) {
			if ds.DataSource.Kind != model.DataSourceFilesystem {
				continue
			}
			root := filepath.Clean(ds.DataSource.Identifier)
			for _, f := range r.candidates(root, glob) {
				if !within(root, f) || !exists(f) {
					continue
				}
				out = append(out, model.Finding{
					RuleID:   r.ID(),
					Severity: model.SeverityHigh,
					NodeID:   s.ID,
					Title:    fmt.Sprintf("%s can read %s", serverName(s), f),
					Message: fmt.Sprintf("The credential file is inside %s, which the server can access; "+
						"one prompt injection is enough to exfiltrate it. Narrow the root.", root),
				})
			}
		}
	}
	return out
}

// candidates lists credential files to look for: well-known files under
// the home directory plus a .env file at the root itself.
func (r CredentialFiles) candidates(root string, glob func(string) []string) []string {
	home := filepath.Clean(r.Home)
	files := []string{
		filepath.Join(home, ".aws", "credentials"),
		filepath.Join(home, ".netrc"),
		filepath.Join(home, ".pgpass"),
		filepath.Join(root, ".env"),
	}
	for _, k := range glob(filepath.Join(home, ".ssh", "id_*")) {
		if !strings.HasSuffix(k, ".pub") { // public keys are not secrets
			files = append(files, k)
		}
	}
	return files
}

// within reports whether path is root itself or below it.
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
