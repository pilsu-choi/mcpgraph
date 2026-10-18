// Package watch re-scans AI app config files when they change on disk.
package watch

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/pilsuchoi/mcpgraph/internal/collector/config"
	"github.com/pilsuchoi/mcpgraph/internal/model"
)

// DefaultDebounce is how long to wait after the last change to a file
// before re-scanning it. Editors often write a file in several steps
// (truncate, write, rename); waiting lets those settle into one scan.
const DefaultDebounce = 300 * time.Millisecond

// Watcher is a collector that runs until its context is cancelled and
// sends a fresh Event for a config file each time it changes.
type Watcher struct {
	home       string
	candidates []config.Candidate
	debounce   time.Duration
	log        *slog.Logger
}

// New returns a Watcher for the given candidate config files.
func New(home string, candidates []config.Candidate, debounce time.Duration, log *slog.Logger) *Watcher {
	return &Watcher{home: home, candidates: candidates, debounce: debounce, log: log}
}

// Name implements collector.Collector.
func (w *Watcher) Name() string { return "watch" }

// Collect implements collector.Collector.
//
// We watch each config file's parent directory rather than the file
// itself: many editors save by writing a temp file and renaming it over
// the original, which would silently end a watch on the old file.
func (w *Watcher) Collect(ctx context.Context, out chan<- model.Event) error {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create file watcher: %w", err)
	}
	// defer runs when Collect returns, however it returns.
	defer fw.Close()

	byPath := map[string]config.Candidate{}
	dirs := map[string]bool{}
	for _, c := range w.candidates {
		byPath[c.Path] = c
		dirs[filepath.Dir(c.Path)] = true
	}
	for dir := range dirs {
		if _, err := os.Stat(dir); err != nil {
			// Limitation: a config directory created after startup is
			// picked up by the daemon's periodic full re-scan instead.
			w.log.Debug("config dir does not exist, not watching", "dir", dir)
			continue
		}
		if err := fw.Add(dir); err != nil {
			w.log.Warn("cannot watch config dir", "dir", dir, "err", err)
			continue
		}
		w.log.Debug("watching", "dir", dir)
	}

	// One timer for all files: every change pushes the deadline back,
	// and when it fires we re-scan every file that changed meanwhile.
	timer := time.NewTimer(w.debounce)
	timer.Stop()
	pending := map[string]bool{}

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-fw.Events:
			if !ok {
				return nil
			}
			if _, watched := byPath[ev.Name]; !watched {
				continue
			}
			w.log.Debug("config file event", "path", ev.Name, "op", ev.Op.String())
			pending[ev.Name] = true
			// Since Go 1.23, Reset on a stopped or fired timer is safe
			// without draining its channel first.
			timer.Reset(w.debounce)
		case err, ok := <-fw.Errors:
			if !ok {
				return nil
			}
			w.log.Warn("file watcher error", "err", err)
		case <-timer.C:
			for path := range pending {
				ev := config.ScanCandidate(byPath[path], w.home)
				select {
				case out <- ev:
				case <-ctx.Done():
					return nil
				}
			}
			// clear empties the map in place (Go 1.21+).
			clear(pending)
		}
	}
}
