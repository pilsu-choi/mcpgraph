// Package graph builds the graph from collector events, links processes to
// servers, and diffs snapshots.
package graph

import (
	"context"
	"sort"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// Builder merges collector events into a graph.
//
// A Builder is NOT safe for concurrent use, and that is the point: exactly
// one goroutine (the one running Run) owns it. Collectors never touch the
// graph; they send Events over a channel. This "single writer" design means
// the graph needs no mutex. Other goroutines only ever see finished,
// immutable *model.Graph values.
type Builder struct {
	// sources holds the latest Event per Event.Source. A new event from a
	// source replaces the old one, which is how removals are handled.
	sources map[string]model.Event
}

// NewBuilder returns an empty Builder.
func NewBuilder() *Builder {
	return &Builder{sources: map[string]model.Event{}}
}

// Apply records an event, replacing any earlier event from the same source.
func (b *Builder) Apply(ev model.Event) {
	b.sources[ev.Source] = ev
}

// Run reads events until in is closed or ctx is cancelled. After each batch
// of events it calls publish with a freshly built graph.
//
// "Batch" means: after one event arrives, Run also takes any events that
// are already waiting, then publishes once. During a full scan this avoids
// rebuilding the graph for every single config file.
func (b *Builder) Run(ctx context.Context, in <-chan model.Event, publish func(*model.Graph)) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-in:
			// ok is false once the channel is closed and drained.
			if !ok {
				return
			}
			b.Apply(ev)
			b.drain(in)
			publish(b.Graph())
		}
	}
}

// drain applies every event already buffered in the channel without
// blocking. The default branch of a select runs when no other case is
// ready, which makes the receive non-blocking.
func (b *Builder) drain(in <-chan model.Event) {
	for {
		select {
		case ev, ok := <-in:
			if !ok {
				return
			}
			b.Apply(ev)
		default:
			return
		}
	}
}

// Graph builds a new graph from the current events. The result shares no
// memory with the Builder, so it can be handed to other goroutines.
func (b *Builder) Graph() *model.Graph {
	names := make([]string, 0, len(b.sources))
	for name := range b.sources {
		names = append(names, name)
	}
	sort.Strings(names)

	nodes := map[string]model.Node{}
	edges := map[model.Edge]bool{} // Edge is a comparable struct, so it can be a map key
	g := &model.Graph{}
	for _, name := range names {
		ev := b.sources[name]
		for _, n := range ev.Nodes {
			// The same node (e.g. a shared data source) can come from two
			// sources; dedupe by ID, first source in sorted order wins.
			if _, dup := nodes[n.ID]; !dup {
				nodes[n.ID] = n
			}
		}
		for _, e := range ev.Edges {
			edges[e] = true
		}
		g.Warnings = append(g.Warnings, ev.Warnings...)
	}

	for _, n := range nodes {
		g.Nodes = append(g.Nodes, n)
	}
	sort.Slice(g.Nodes, func(i, j int) bool { return g.Nodes[i].ID < g.Nodes[j].ID })

	for e := range edges {
		// Drop dangling edges, e.g. CHILD_OF to a parent we did not record.
		if _, ok := nodes[e.From]; !ok {
			continue
		}
		if _, ok := nodes[e.To]; !ok {
			continue
		}
		g.Edges = append(g.Edges, e)
	}

	linkProcesses(g)
	linkDecisions(g)
	sortEdges(g.Edges)
	// Snapshot deep-copies the nodes so the result does not share pointer
	// fields with the events still stored in b.sources.
	return g.Snapshot()
}

func sortEdges(edges []model.Edge) {
	sort.Slice(edges, func(i, j int) bool {
		a, b := edges[i], edges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.To < b.To
	})
}
