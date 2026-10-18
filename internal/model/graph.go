package model

import "slices"

// Graph is the full picture of one scan.
//
// Nodes and Edges are kept as sorted slices (not maps) so that JSON output
// is deterministic and two graphs can be compared byte for byte. The graphs
// we build are small (tens to hundreds of nodes), so the lookup helpers
// below use plain loops instead of maintaining an index.
type Graph struct {
	Nodes    []Node   `json:"nodes"`
	Edges    []Edge   `json:"edges"`
	Warnings []string `json:"warnings,omitempty"`
}

// Node returns the node with the given ID.
// The second return value is false when no such node exists (the Go
// "comma ok" idiom, like dict.get() returning None in Python).
func (g *Graph) Node(id string) (Node, bool) {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}

// NodesOfKind returns all nodes of the given kind, in graph order.
func (g *Graph) NodesOfKind(kind NodeKind) []Node {
	var out []Node
	for _, n := range g.Nodes {
		if n.Kind == kind {
			out = append(out, n)
		}
	}
	return out
}

// Out returns the nodes reached from id over edges of the given kind.
func (g *Graph) Out(id string, kind EdgeKind) []Node {
	var out []Node
	for _, e := range g.Edges {
		if e.From == id && e.Kind == kind {
			if n, ok := g.Node(e.To); ok {
				out = append(out, n)
			}
		}
	}
	return out
}

// In returns the nodes that point to id over edges of the given kind.
func (g *Graph) In(id string, kind EdgeKind) []Node {
	var out []Node
	for _, e := range g.Edges {
		if e.To == id && e.Kind == kind {
			if n, ok := g.Node(e.From); ok {
				out = append(out, n)
			}
		}
	}
	return out
}

// Snapshot returns a deep copy of the graph.
//
// The builder never mutates a graph after publishing it, but callers that
// hand a graph to another goroutine should still pass a Snapshot: Go slices
// and pointers share their backing memory, so a shallow copy would let two
// goroutines see the same data.
func (g *Graph) Snapshot() *Graph {
	out := &Graph{
		Nodes:    make([]Node, len(g.Nodes)),
		Edges:    slices.Clone(g.Edges),
		Warnings: slices.Clone(g.Warnings),
	}
	for i, n := range g.Nodes {
		out.Nodes[i] = n.clone()
	}
	return out
}

// clone deep-copies a node. Each pointer field is copied into a new value
// so the copy shares no memory with the original.
func (n Node) clone() Node {
	// n is already a copy (method has a value receiver), but its pointer
	// fields still point at the original structs, so copy each one.
	if n.Host != nil {
		h := *n.Host
		n.Host = &h
	}
	if n.App != nil {
		a := *n.App
		n.App = &a
	}
	if n.Server != nil {
		s := *n.Server
		s.Args = slices.Clone(s.Args)
		s.EnvKeys = slices.Clone(s.EnvKeys)
		n.Server = &s
	}
	if n.Process != nil {
		p := *n.Process
		p.Args = slices.Clone(p.Args)
		n.Process = &p
	}
	if n.Credential != nil {
		c := *n.Credential
		n.Credential = &c
	}
	if n.DataSource != nil {
		d := *n.DataSource
		n.DataSource = &d
	}
	return n
}
