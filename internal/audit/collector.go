package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// DefaultGraphLimit is how many recent decisions a scan puts in the graph.
const DefaultGraphLimit = 50

// Collector turns the newest decisions of the audit log into Decision
// nodes. It sends them to the graph builder like any other collector; the
// builder draws the DECIDED_ON edges, because only it sees the server and
// app nodes from the other collectors.
type Collector struct {
	Path  string
	Limit int // newest N decisions; 0 means DefaultGraphLimit
}

// Name implements collector.Collector.
func (c Collector) Name() string { return "audit" }

// Collect implements collector.Collector.
func (c Collector) Collect(ctx context.Context, out chan<- model.Event) error {
	limit := c.Limit
	if limit <= 0 {
		limit = DefaultGraphLimit
	}
	ev := model.Event{Source: "audit"}
	res, err := ReadFile(c.Path, Filter{Limit: limit})
	if err != nil {
		// An unreadable audit log must not fail the posture scan.
		ev.Warnings = append(ev.Warnings, err.Error())
	}
	if res.Skipped > 0 {
		ev.Warnings = append(ev.Warnings, fmt.Sprintf("audit log %s: skipped %d malformed lines", c.Path, res.Skipped))
	}
	for _, d := range res.Decisions {
		ev.Nodes = append(ev.Nodes, DecisionNode(d))
	}
	select {
	case out <- ev:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// DecisionNode wraps a decision in a graph node. The ID hashes the fields
// that make a decision unique, so replaying the same log gives the same
// IDs.
func DecisionNode(d model.Decision) model.Node {
	key := d.Time.UTC().Format(time.RFC3339Nano) + "|" + d.Source + "|" + d.Agent + "|" + d.App + "|" +
		d.Tool + "|" + d.RuleID + "|" + d.Summary
	label := string(d.Verdict) + " " + d.Tool
	if d.RuleID != "" {
		label += " (" + d.RuleID + ")"
	}
	return model.Node{
		ID:       model.NodeID(model.KindDecision, key),
		Kind:     model.KindDecision,
		Label:    label,
		Decision: &d,
	}
}
