package graph

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pilsuchoi/mcpgraph/internal/model"
)

// Change types.
const (
	Added   = "added"
	Removed = "removed"
	Changed = "changed"
)

// Change is one difference between two snapshots.
type Change struct {
	Type    string         `json:"type"`
	Kind    model.NodeKind `json:"kind"`
	NodeID  string         `json:"node_id"`
	Label   string         `json:"label"`
	Message string         `json:"message"`
}

// Diff compares two graphs node by node. Stable node IDs are what make
// this possible: the same server has the same ID in both graphs.
// old may be nil (first snapshot), in which case everything is "added".
func Diff(old, cur *model.Graph) []Change {
	if old == nil {
		old = &model.Graph{}
	}
	oldNodes := index(old)
	curNodes := index(cur)

	var out []Change
	// Walk cur.Nodes (a sorted slice) rather than the map so the output
	// order is deterministic.
	for _, n := range cur.Nodes {
		before, existed := oldNodes[n.ID]
		switch {
		case !existed:
			out = append(out, change(Added, n))
		case !sameNode(before, n):
			out = append(out, change(Changed, n))
		}
	}
	for _, n := range old.Nodes {
		if _, still := curNodes[n.ID]; !still {
			out = append(out, change(Removed, n))
		}
	}
	return out
}

func index(g *model.Graph) map[string]model.Node {
	m := make(map[string]model.Node, len(g.Nodes))
	for _, n := range g.Nodes {
		m[n.ID] = n
	}
	return m
}

// sameNode compares two nodes by their JSON form, ignoring the scan time
// on Host nodes, which changes on every scan.
func sameNode(a, b model.Node) bool {
	return diffKey(a) == diffKey(b)
}

// diffKey returns the form of a node that Diff compares.
func diffKey(n model.Node) string {
	if n.Host != nil {
		h := *n.Host // copy, so the caller's node is not modified
		h.ScannedAt = time.Time{}
		n.Host = &h
	}
	b, err := json.Marshal(n)
	if err != nil {
		// Node contains only strings, numbers, slices and times, which
		// always marshal; fall back to fmt just in case.
		return fmt.Sprintf("%+v", n)
	}
	return string(b)
}

func change(typ string, n model.Node) Change {
	return Change{Type: typ, Kind: n.Kind, NodeID: n.ID, Label: n.Label, Message: message(typ, n)}
}

// message writes a sentence a human can read in a log line.
func message(typ string, n model.Node) string {
	switch {
	case n.Kind == model.KindMCPServer && typ == Added:
		return fmt.Sprintf("new MCP server %q added to %s config", n.Server.Name, n.Server.App)
	case n.Kind == model.KindMCPServer && typ == Removed:
		return fmt.Sprintf("MCP server %q removed from %s config", n.Server.Name, n.Server.App)
	case n.Kind == model.KindMCPServer && typ == Changed:
		return fmt.Sprintf("MCP server %q in %s config changed", n.Server.Name, n.Server.App)
	case n.Kind == model.KindCredential && typ == Added:
		return fmt.Sprintf("new %s credential %q at %s", n.Credential.Kind, n.Credential.KeyName, n.Credential.Location)
	case n.Kind == model.KindAIApp && typ == Added:
		return fmt.Sprintf("new AI app config found: %s (%s)", n.App.Name, n.App.ConfigPath)
	case n.Kind == model.KindProcess && typ == Added:
		return fmt.Sprintf("process started: %s", n.Process.Cmdline)
	case n.Kind == model.KindProcess && typ == Removed:
		return fmt.Sprintf("process exited: %s", n.Process.Cmdline)
	}
	return fmt.Sprintf("%s %q %s", n.Kind, n.Label, typ)
}
