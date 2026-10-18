// Package collector defines the interface every collector implements and
// the small Host collector.
package collector

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// Collector discovers part of the graph and sends it as Events.
//
// Collect must return when ctx is cancelled. It must not close out: the
// channel is shared by several collectors, so only the code that created
// it (the scan orchestrator) closes it, after every collector has returned.
type Collector interface {
	Name() string
	Collect(ctx context.Context, out chan<- model.Event) error
}

// Host emits a single Host node describing this machine.
type Host struct {
	// Now is the clock; tests replace it to get stable output.
	Now func() time.Time
}

// Name implements Collector.
func (Host) Name() string { return "host" }

// Collect implements Collector.
func (h Host) Collect(ctx context.Context, out chan<- model.Event) error {
	now := time.Now
	if h.Now != nil {
		now = h.Now
	}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	ev := model.Event{
		Source: "host",
		Nodes: []model.Node{{
			ID:    model.NodeID(model.KindHost, hostname),
			Kind:  model.KindHost,
			Label: hostname,
			Host: &model.Host{
				Hostname:  hostname,
				OS:        runtime.GOOS,
				Arch:      runtime.GOARCH,
				ScannedAt: now().UTC().Truncate(time.Second),
			},
		}},
	}
	select {
	case out <- ev:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
