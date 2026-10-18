package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

func node(id string, kind model.NodeKind) model.Node {
	n := model.Node{ID: id, Kind: kind, Label: id}
	if kind == model.KindMCPServer {
		n.Server = &model.MCPServer{Name: id, Transport: model.TransportStdio}
	}
	return n
}

func TestBuilderReplacesPerSource(t *testing.T) {
	b := NewBuilder()
	b.Apply(model.Event{Source: "config:a", Nodes: []model.Node{node("s1", model.KindMCPServer), node("s2", model.KindMCPServer)}})
	b.Apply(model.Event{Source: "config:b", Nodes: []model.Node{node("s3", model.KindMCPServer)}})
	if got := len(b.Graph().Nodes); got != 3 {
		t.Fatalf("nodes = %d, want 3", got)
	}

	// A new event from config:a replaces its earlier contribution: s2 is gone.
	b.Apply(model.Event{Source: "config:a", Nodes: []model.Node{node("s1", model.KindMCPServer)}})
	g := b.Graph()
	if _, ok := g.Node("s2"); ok {
		t.Error("s2 should have been removed")
	}
	if _, ok := g.Node("s3"); !ok {
		t.Error("s3 from another source should remain")
	}
}

func TestBuilderDedupesAndDropsDanglingEdges(t *testing.T) {
	b := NewBuilder()
	shared := node("ds", model.KindDataSource)
	b.Apply(model.Event{
		Source: "config:a",
		Nodes:  []model.Node{node("s1", model.KindMCPServer), shared},
		Edges: []model.Edge{
			{From: "s1", To: "ds", Kind: model.EdgeAccesses},
			{From: "s1", To: "missing", Kind: model.EdgeAccesses},
		},
	})
	b.Apply(model.Event{
		Source: "config:b",
		Nodes:  []model.Node{node("s2", model.KindMCPServer), shared},
		Edges:  []model.Edge{{From: "s2", To: "ds", Kind: model.EdgeAccesses}},
	})
	g := b.Graph()
	if len(g.Nodes) != 3 {
		t.Errorf("nodes = %d, want 3 (shared data source deduped)", len(g.Nodes))
	}
	if len(g.Edges) != 2 {
		t.Errorf("edges = %+v, want 2 (dangling edge dropped)", g.Edges)
	}
}

func TestBuilderOutputIsDeterministic(t *testing.T) {
	build := func() string {
		b := NewBuilder()
		for i := range 5 {
			b.Apply(model.Event{Source: fmt.Sprintf("s%d", i), Nodes: []model.Node{node(fmt.Sprintf("n%d", i), model.KindMCPServer)}})
		}
		// Compare JSON, not %+v: %+v prints pointer addresses.
		raw, err := json.Marshal(b.Graph())
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	first, second := build(), build()
	if first != second {
		t.Fatal("graph order depends on map iteration order")
	}
}

func TestBuilderGraphSharesNoMemory(t *testing.T) {
	b := NewBuilder()
	b.Apply(model.Event{Source: "a", Nodes: []model.Node{node("s1", model.KindMCPServer)}})
	g := b.Graph()
	g.Nodes[0].Server.Name = "mutated"
	if b.Graph().Nodes[0].Server.Name != "s1" {
		t.Fatal("published graph shares memory with builder state")
	}
}

// TestRunConcurrentProducers sends events from many goroutines. Run with
// -race to prove that the single-writer design needs no locks.
func TestRunConcurrentProducers(t *testing.T) {
	in := make(chan model.Event)
	b := NewBuilder()
	var last *model.Graph
	done := make(chan struct{})
	go func() {
		defer close(done)
		b.Run(context.Background(), in, func(g *model.Graph) { last = g })
	}()

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := fmt.Sprintf("n%d", i)
			in <- model.Event{Source: id, Nodes: []model.Node{node(id, model.KindMCPServer)}}
		}()
	}
	wg.Wait()
	close(in)
	<-done

	if last == nil || len(last.Nodes) != 20 {
		t.Fatalf("final graph has wrong size: %+v", last)
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		NewBuilder().Run(ctx, make(chan model.Event), func(*model.Graph) {})
	}()
	cancel()
	<-done // would hang (and the test time out) if Run ignored ctx
}
