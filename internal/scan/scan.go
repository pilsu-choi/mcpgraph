// Package scan wires collectors and the graph builder together for a
// one-shot scan.
package scan

import (
	"context"

	"golang.org/x/sync/errgroup"

	"github.com/pilsu-choi/mcpgraph/internal/collector"
	"github.com/pilsu-choi/mcpgraph/internal/graph"
	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// Run starts every collector in its own goroutine, feeds their events to a
// single graph builder, and returns the finished graph.
func Run(ctx context.Context, collectors []collector.Collector) (*model.Graph, error) {
	events := make(chan model.Event, len(collectors))

	// errgroup.WithContext returns a derived ctx that is cancelled as soon
	// as any goroutine in the group returns an error, so one failing
	// collector stops the others. g.Wait() waits for all of them and
	// returns the first error.
	g, gctx := errgroup.WithContext(ctx)
	for _, c := range collectors {
		g.Go(func() error {
			return c.Collect(gctx, events)
		})
	}

	// The builder runs in its own goroutine. result is written only by
	// that goroutine and read only after <-done, so no lock is needed:
	// closing a channel "happens before" a receive that sees it closed.
	b := graph.NewBuilder()
	result := b.Graph()
	done := make(chan struct{})
	go func() {
		defer close(done)
		b.Run(ctx, events, func(gr *model.Graph) { result = gr })
	}()

	err := g.Wait()
	// Every collector has returned, so nobody sends anymore: this
	// goroutine created the channel and is the one that closes it.
	close(events)
	<-done
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Collect runs every collector concurrently and returns all their events
// without building a graph. The daemon uses it for its initial scan.
func Collect(ctx context.Context, collectors []collector.Collector) ([]model.Event, error) {
	events := make(chan model.Event, len(collectors))
	g, gctx := errgroup.WithContext(ctx)
	for _, c := range collectors {
		g.Go(func() error { return c.Collect(gctx, events) })
	}

	// Close the channel once every collector is done, so the range loop
	// below ends. This goroutine is the only one that closes it.
	var err error
	go func() {
		err = g.Wait()
		close(events)
	}()

	var out []model.Event
	for ev := range events {
		out = append(out, ev)
	}
	// err was written before close(events), and the range loop has seen
	// the close, so reading err here is race-free.
	return out, err
}
