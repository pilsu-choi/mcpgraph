// Package report renders a graph and its findings for humans (tables) and
// machines (JSON, Mermaid).
package report

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/pilsuchoi/mcpgraph/internal/model"
)

// Inventory writes a human-readable list of apps, servers and processes.
func Inventory(w io.Writer, g *model.Graph) error {
	// tabwriter aligns columns separated by '\t'. Flush must be called at
	// the end; defer runs it when the function returns, even on early
	// returns.
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer tw.Flush()

	for _, h := range g.NodesOfKind(model.KindHost) {
		fmt.Fprintf(tw, "Host: %s (%s/%s)\n\n", h.Host.Hostname, h.Host.OS, h.Host.Arch)
	}

	apps := g.NodesOfKind(model.KindAIApp)
	fmt.Fprintf(tw, "AI APPS (%d)\n", len(apps))
	if len(apps) > 0 {
		fmt.Fprintln(tw, "APP\tSERVERS\tCONFIG")
		for _, a := range apps {
			n := len(g.Out(a.ID, model.EdgeConfigures))
			fmt.Fprintf(tw, "%s\t%d\t%s\n", a.App.Name, n, a.App.ConfigPath)
		}
	}

	servers := g.NodesOfKind(model.KindMCPServer)
	fmt.Fprintf(tw, "\nMCP SERVERS (%d)\n", len(servers))
	if len(servers) > 0 {
		fmt.Fprintln(tw, "APP\tSERVER\tTRANSPORT\tCOMMAND / URL\tSECRETS\tACCESSES\tPIDS")
		for _, s := range servers {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
				s.Server.App, s.Server.Name, s.Server.Transport, truncate(target(s.Server), 60),
				len(g.Out(s.ID, model.EdgeUsesSecret)), accesses(g, s.ID), pids(g, s.ID))
		}
	}

	procs := g.NodesOfKind(model.KindProcess)
	if len(procs) > 0 {
		fmt.Fprintf(tw, "\nPROCESSES (%d)\n", len(procs))
		fmt.Fprintln(tw, "PID\tPPID\tAPP\tMCP\tCOMMAND")
		for _, p := range procs {
			mcp := ""
			if p.Process.MCPLike {
				mcp = "yes"
			}
			fmt.Fprintf(tw, "%d\t%d\t%s\t%s\t%s\n", p.Process.PID, p.Process.PPID, p.Process.AIApp, mcp, truncate(p.Process.Cmdline, 70))
		}
	}

	if len(g.Warnings) > 0 {
		fmt.Fprintf(tw, "\nWARNINGS (%d)\n", len(g.Warnings))
		for _, warn := range g.Warnings {
			fmt.Fprintf(tw, "- %s\n", warn)
		}
	}
	return nil
}

// target is what a server runs or connects to.
func target(s *model.MCPServer) string {
	if s.URL != "" {
		return s.URL
	}
	return strings.TrimSpace(s.Command + " " + strings.Join(s.Args, " "))
}

func accesses(g *model.Graph, serverID string) string {
	var parts []string
	for _, ds := range g.Out(serverID, model.EdgeAccesses) {
		parts = append(parts, ds.DataSource.Kind+":"+ds.DataSource.Identifier)
	}
	if len(parts) == 0 {
		return "-"
	}
	return truncate(strings.Join(parts, ", "), 50)
}

func pids(g *model.Graph, serverID string) string {
	var parts []string
	for _, p := range g.Out(serverID, model.EdgeRunsAs) {
		parts = append(parts, fmt.Sprint(p.Process.PID))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ",")
}

// truncate shortens s to at most n runes (not bytes, so multi-byte
// characters are never cut in half).
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
