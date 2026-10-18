package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// Events writes audit decisions as a table, oldest first.
func Events(w io.Writer, decisions []model.Decision) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "DECISIONS (%d)\n", len(decisions))
	if len(decisions) > 0 {
		fmt.Fprintln(tw, "TIME (UTC)\tVERDICT\tRULE\tSOURCE\tAGENT\tTOOL\tACTION")
		for _, d := range decisions {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				d.Time.UTC().Format("2006-01-02 15:04:05"), strings.ToUpper(string(d.Verdict)), orDash(d.RuleID),
				d.Source, agentLabel(d), d.Tool, truncate(d.Summary, 70))
		}
	}
	return tw.Flush()
}

// EventsJSONL writes one decision per line, the same format as the audit
// log, so the output can be piped to jq or another tool.
func EventsJSONL(w io.Writer, decisions []model.Decision) error {
	enc := json.NewEncoder(w) // Encode adds the newline after each value
	for _, d := range decisions {
		if err := enc.Encode(d); err != nil {
			return err
		}
	}
	return nil
}

// writeDecisionRows lists Decision nodes in the inventory, oldest first,
// with the node each one points at.
func writeDecisionRows(tw *tabwriter.Writer, g *model.Graph, nodes []model.Node) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Decision.Time.Before(nodes[j].Decision.Time) })
	fmt.Fprintln(tw, "TIME (UTC)\tVERDICT\tRULE\tDECIDED ON\tTOOL\tACTION")
	for _, n := range nodes {
		d := n.Decision
		on := "-"
		if targets := g.Out(n.ID, model.EdgeDecidedOn); len(targets) > 0 {
			on = nodeName(targets[0])
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			d.Time.UTC().Format("2006-01-02 15:04:05"), strings.ToUpper(string(d.Verdict)), orDash(d.RuleID),
			on, d.Tool, truncate(d.Summary, 60))
	}
}

func nodeName(n model.Node) string {
	switch n.Kind {
	case model.KindMCPServer:
		return n.Server.App + " / " + n.Server.Name
	case model.KindAIApp:
		return n.App.Name
	}
	return n.Label
}

func agentLabel(d model.Decision) string {
	if d.App != "" && !strings.EqualFold(d.App, d.Agent) {
		return d.Agent + " (" + d.App + ")"
	}
	return d.Agent
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
