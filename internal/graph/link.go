package graph

import (
	"path/filepath"
	"strings"

	"github.com/pilsuchoi/mcpgraph/internal/model"
)

// linkProcesses adds RUNS_AS edges from declared stdio MCP servers to the
// running processes that match them.
//
// When the same server is declared by two apps (a common copy-paste), the
// process is linked to the server of the app it actually descends from, if
// that can be told from the process tree.
func linkProcesses(g *model.Graph) {
	servers := g.NodesOfKind(model.KindMCPServer)
	for _, p := range g.NodesOfKind(model.KindProcess) {
		if p.Process.AIApp != "" {
			continue
		}
		var matched []model.Node
		for _, s := range servers {
			if Matches(s.Server, p.Process) {
				matched = append(matched, s)
			}
		}
		matched = preferOwningApp(g, p, matched)
		for _, s := range matched {
			g.Edges = append(g.Edges, model.Edge{From: s.ID, To: p.ID, Kind: model.EdgeRunsAs})
		}
	}
}

// preferOwningApp keeps only servers whose App equals the AI app the
// process descends from. If that filter removes everything, all matches
// are kept.
func preferOwningApp(g *model.Graph, p model.Node, matched []model.Node) []model.Node {
	if len(matched) < 2 {
		return matched
	}
	app := ancestorApp(g, p.ID)
	if app == "" {
		return matched
	}
	var out []model.Node
	for _, s := range matched {
		if s.Server.App == app {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return matched
	}
	return out
}

// ancestorApp walks CHILD_OF edges upwards and returns the first AI app
// name found.
func ancestorApp(g *model.Graph, id string) string {
	seen := map[string]bool{}
	for !seen[id] {
		seen[id] = true
		parents := g.Out(id, model.EdgeChildOf)
		if len(parents) == 0 {
			return ""
		}
		if parents[0].Process.AIApp != "" {
			return parents[0].Process.AIApp
		}
		id = parents[0].ID
	}
	return ""
}

// Matches reports whether process p runs server s.
//
// Every non-flag argument of the server must appear in the process's
// arguments (either exactly or as the last path element). If the server
// has no such arguments, its command must appear instead. Both sides are
// redacted the same way, so masked secrets still compare equal.
func Matches(s *model.MCPServer, p *model.Process) bool {
	if s.Transport != model.TransportStdio || s.Command == "" {
		return false
	}
	var required []string
	for _, a := range s.Args {
		if !strings.HasPrefix(a, "-") {
			required = append(required, a)
		}
	}
	if len(required) == 0 {
		required = []string{filepath.Base(s.Command)}
	}
	for _, r := range required {
		if !hasArg(p.Args, r) {
			return false
		}
	}
	return true
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want || strings.HasSuffix(a, "/"+want) {
			return true
		}
	}
	return false
}
