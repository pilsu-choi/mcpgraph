// Package config discovers AI app MCP config files and turns them into
// graph events.
//
// Adding a new app means adding one parser file (see cursor.go for the
// smallest example), a line in paths.go and fixtures in testdata/configs.
package config

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// MaxConfigSize caps how much of a config file we read. ~/.claude.json can
// be several megabytes because it also stores app state.
const MaxConfigSize = 32 << 20 // 32 MiB

// Collector reads every candidate config file once.
type Collector struct {
	home       string
	candidates []Candidate
}

// New returns a Collector for the given home directory and GOOS.
func New(home, goos string) *Collector {
	return &Collector{home: home, candidates: Candidates(home, goos)}
}

// Name implements collector.Collector.
func (c *Collector) Name() string { return "config" }

// Candidates returns the config paths this collector checks.
func (c *Collector) Candidates() []Candidate { return c.candidates }

// Collect sends one Event per candidate file, even for files that do not
// exist: an empty event clears anything that file contributed earlier.
func (c *Collector) Collect(ctx context.Context, out chan<- model.Event) error {
	for _, cand := range c.candidates {
		ev := ScanCandidate(cand, c.home)
		// select waits for whichever happens first: the builder accepts
		// the event, or the context is cancelled. Without the ctx case a
		// send could block forever after the builder has stopped.
		select {
		case out <- ev:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// SourceFor returns the Event.Source used for a config path.
func SourceFor(path string) string { return "config:" + path }

// ScanCandidate reads and parses one config file. Errors become warnings on
// the event: a broken config for one app must not fail the whole scan.
func ScanCandidate(cand Candidate, home string) model.Event {
	ev := model.Event{Source: SourceFor(cand.Path)}

	data, err := readLimited(cand.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return ev
	}
	if err != nil {
		ev.Warnings = append(ev.Warnings, fmt.Sprintf("read %s config %s: %v", cand.App, cand.Path, err))
		return ev
	}

	d := newDeriver(home)
	appID := d.app(cand.App, cand.Path)

	servers, err := cand.parser.Parse(data)
	if err != nil {
		ev.Warnings = append(ev.Warnings, fmt.Sprintf("parse %s config %s: %v", cand.App, cand.Path, err))
	}
	for _, s := range servers {
		d.server(appID, cand.App, cand.Path, s)
	}

	ev.Nodes = d.nodes
	ev.Edges = d.edges
	return ev
}

// readLimited reads a file but refuses anything larger than MaxConfigSize.
func readLimited(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > MaxConfigSize {
		return nil, fmt.Errorf("file is %d bytes, larger than limit %d", info.Size(), MaxConfigSize)
	}
	return os.ReadFile(path)
}
