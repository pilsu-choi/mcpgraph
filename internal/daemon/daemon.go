// Package daemon runs mcpgraph continuously: it watches config files,
// re-scans periodically, stores snapshots and logs what changed.
package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/pilsu-choi/mcpgraph/internal/collector"
	"github.com/pilsu-choi/mcpgraph/internal/collector/config"
	"github.com/pilsu-choi/mcpgraph/internal/collector/process"
	"github.com/pilsu-choi/mcpgraph/internal/collector/watch"
	"github.com/pilsu-choi/mcpgraph/internal/graph"
	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/rules"
	"github.com/pilsu-choi/mcpgraph/internal/scan"
	"github.com/pilsu-choi/mcpgraph/internal/sink"
	"github.com/pilsu-choi/mcpgraph/internal/store"
)

// Options configures a Daemon.
type Options struct {
	Home      string
	GOOS      string
	Interval  time.Duration // full re-scan period
	Debounce  time.Duration // config watcher debounce
	Processes bool          // include the process collector
	Store     *store.Store
	Sink      sink.Sink // where snapshots are forwarded; nil means sink.Nop
	Log       *slog.Logger
	Now       func() time.Time
}

// State is the latest published result. It is immutable once published.
type State struct {
	Graph    *model.Graph
	Findings []model.Finding
	TakenAt  time.Time
}

// Daemon owns the long-running pipeline.
type Daemon struct {
	opts Options
	// current is written by the builder goroutine and read by API
	// handlers. atomic.Pointer swaps the whole *State at once, so a reader
	// always sees a complete State and never needs a lock.
	current atomic.Pointer[State]
	last    *model.Graph // only touched by the builder goroutine
}

// New returns a Daemon. Call Run to start it.
func New(opts Options) *Daemon {
	if opts.Sink == nil {
		opts.Sink = sink.Nop{}
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Debounce == 0 {
		opts.Debounce = watch.DefaultDebounce
	}
	if opts.Interval == 0 {
		opts.Interval = time.Minute
	}
	return &Daemon{opts: opts}
}

// State returns the latest state, or nil before the first scan finished.
func (d *Daemon) State() *State {
	return d.current.Load()
}

// Current returns the latest graph and findings; ok is false before the
// first scan finished. It satisfies api.StateSource.
func (d *Daemon) Current() (*model.Graph, []model.Finding, bool) {
	st := d.current.Load()
	if st == nil {
		return nil, nil, false
	}
	return st.Graph, st.Findings, true
}

// Run blocks until ctx is cancelled or a component fails.
func (d *Daemon) Run(ctx context.Context) error {
	last, err := d.opts.Store.Latest(ctx)
	if err != nil {
		return fmt.Errorf("load latest snapshot: %w", err)
	}
	d.last = last

	cfg := config.New(d.opts.Home, d.opts.GOOS)
	oneShot := []collector.Collector{collector.Host{Now: d.opts.Now}, cfg}
	if d.opts.Processes {
		oneShot = append(oneShot, process.New())
	}

	// Initial scan: apply every event before publishing, so the first
	// snapshot is complete instead of growing source by source.
	b := graph.NewBuilder()
	initial, err := scan.Collect(ctx, oneShot)
	if err != nil {
		return err
	}
	for _, ev := range initial {
		b.Apply(ev)
	}
	d.publish(ctx, b.Graph())

	events := make(chan model.Event, 16)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return watch.New(d.opts.Home, cfg.Candidates(), d.opts.Debounce, d.opts.Log).Collect(gctx, events)
	})
	g.Go(func() error {
		return d.poll(gctx, oneShot, events)
	})
	g.Go(func() error {
		// The builder goroutine is the single writer of graph state.
		b.Run(gctx, events, func(gr *model.Graph) { d.publish(gctx, gr) })
		return nil
	})
	return g.Wait()
}

// poll re-runs the one-shot collectors every Interval. This catches what
// the file watcher cannot see: processes, and config dirs created later.
func (d *Daemon) poll(ctx context.Context, collectors []collector.Collector, out chan<- model.Event) error {
	ticker := time.NewTicker(d.opts.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			for _, c := range collectors {
				if err := c.Collect(ctx, out); err != nil {
					if ctx.Err() != nil {
						return nil
					}
					d.opts.Log.Warn("periodic collect failed", "collector", c.Name(), "err", err)
				}
			}
		}
	}
}

// publish evaluates rules, logs the diff against the previous snapshot and
// stores a new snapshot if anything changed. It runs on the builder
// goroutine only.
func (d *Daemon) publish(ctx context.Context, g *model.Graph) {
	now := d.opts.Now()
	findings := rules.Evaluate(g, rules.Default(d.opts.Home))
	d.current.Store(&State{Graph: g, Findings: findings, TakenAt: now})

	changes := graph.Diff(d.last, g)
	if d.last != nil && len(changes) == 0 {
		return
	}
	for _, c := range changes {
		level := slog.LevelInfo
		if c.Kind == model.KindProcess || c.Kind == model.KindHost {
			level = slog.LevelDebug // processes come and go constantly
		}
		d.opts.Log.Log(ctx, level, c.Message, "change", c.Type, "kind", c.Kind, "node_id", c.NodeID)
	}
	for _, w := range g.Warnings {
		d.opts.Log.Warn(w)
	}

	id, err := d.opts.Store.Save(ctx, now, g, findings, changes)
	if err != nil {
		d.opts.Log.Error("save snapshot", "err", err)
		return
	}
	d.last = g
	d.opts.Log.Info("snapshot saved", "id", id, "nodes", len(g.Nodes), "findings", len(findings), "changes", len(changes))

	if err := d.opts.Sink.Send(ctx, g, findings); err != nil {
		d.opts.Log.Warn("sink send failed", "err", err)
	}
}
