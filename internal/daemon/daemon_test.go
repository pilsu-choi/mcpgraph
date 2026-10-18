package daemon

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/store"
	"github.com/pilsu-choi/mcpgraph/internal/testutil"
)

// syncBuffer is a bytes.Buffer safe for use from several goroutines: the
// daemon writes logs while the test reads them.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// waitFor polls cond until it is true or the timeout expires.
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return cond()
}

func startDaemon(t *testing.T, home string) (*Daemon, *syncBuffer, *store.Store) {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "mcpgraph.db"))
	if err != nil {
		t.Fatal(err)
	}
	logs := &syncBuffer{}
	d := New(Options{
		Home:     home,
		GOOS:     "linux",
		Interval: time.Hour, // only the file watcher should trigger re-scans
		Debounce: 100 * time.Millisecond,
		Store:    st,
		Log:      slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	// Cleanup functions run in reverse order: stop the daemon first,
	// then close the store it was writing to.
	t.Cleanup(func() { st.Close() })
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Run returned %v", err)
		}
	})

	if !waitFor(t, 5*time.Second, func() bool { return d.State() != nil }) {
		t.Fatal("daemon never published its first snapshot")
	}
	return d, logs, st
}

// TestConfigEditLogsDiffWithinTwoSeconds is the milestone 7 acceptance test.
func TestConfigEditLogsDiffWithinTwoSeconds(t *testing.T) {
	home := testutil.FakeHome(t)
	_, logs, st := startDaemon(t, home)

	cursorCfg := filepath.Join(home, ".cursor", "mcp.json")
	newCfg := `{"mcpServers": {"notion": {"command": "npx", "args": ["-y", "@notionhq/notion-mcp-server"]}}}`
	start := time.Now()
	if err := os.WriteFile(cursorCfg, []byte(newCfg), 0o600); err != nil {
		t.Fatal(err)
	}

	want := `new MCP server \"notion\" added to Cursor config`
	if !waitFor(t, 2*time.Second, func() bool { return strings.Contains(logs.String(), want) }) {
		t.Fatalf("no diff logged within 2s; logs:\n%s", logs.String())
	}
	t.Logf("diff logged after %v", time.Since(start).Round(time.Millisecond))

	if !strings.Contains(logs.String(), `MCP server \"git\" removed from Cursor config`) {
		t.Error("removal of old servers was not logged")
	}
	// The diff is logged just before the snapshot is saved, so wait.
	saved := waitFor(t, time.Second, func() bool {
		metas, err := st.List(context.Background(), 10)
		return err == nil && len(metas) == 2
	})
	if !saved {
		t.Fatal("want 2 snapshots: initial + after edit")
	}
}

func TestNoSnapshotWhenNothingChanged(t *testing.T) {
	home := testutil.FakeHome(t)
	_, _, st := startDaemon(t, home)

	// Rewrite a config with identical content: an event fires, but the
	// graph is the same, so no new snapshot must be stored.
	path := filepath.Join(home, ".cursor", "mcp.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)

	metas, err := st.List(context.Background(), 10)
	if err != nil || len(metas) != 1 {
		t.Fatalf("snapshots = %d, err = %v; want 1", len(metas), err)
	}
}
