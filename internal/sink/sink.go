// Package sink defines where snapshots could be forwarded.
//
// Shipping data to a central server is out of scope for the MVP (and would
// break hard rule 3, no network egress), so only a no-op exists. The
// interface marks the seam where a real implementation would plug in.
package sink

import (
	"context"

	"github.com/pilsuchoi/mcpgraph/internal/model"
)

// Sink receives every new snapshot.
type Sink interface {
	Send(ctx context.Context, g *model.Graph, findings []model.Finding) error
}

// Nop discards everything.
type Nop struct{}

// Send implements Sink.
func (Nop) Send(context.Context, *model.Graph, []model.Finding) error { return nil }

// Compile-time check that Nop satisfies Sink. Assigning to the blank
// identifier costs nothing at runtime but fails the build if a method is
// missing, since Go has no "implements" keyword.
var _ Sink = Nop{}
