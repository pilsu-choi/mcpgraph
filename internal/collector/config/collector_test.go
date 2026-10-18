package config

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pilsuchoi/mcpgraph/internal/model"
	"github.com/pilsuchoi/mcpgraph/internal/testutil"
)

// collectAll runs the collector and returns every event it sent.
func collectAll(t *testing.T, c *Collector) []model.Event {
	t.Helper()
	// Buffered so Collect never blocks waiting for a reader.
	out := make(chan model.Event, len(c.Candidates()))
	if err := c.Collect(context.Background(), out); err != nil {
		t.Fatal(err)
	}
	close(out)
	var evs []model.Event
	for ev := range out {
		evs = append(evs, ev)
	}
	return evs
}

func TestCollectFakeHome(t *testing.T) {
	home := testutil.FakeHome(t)
	evs := collectAll(t, New(home, "linux"))

	if len(evs) != len(Candidates(home, "linux")) {
		t.Fatalf("got %d events, want one per candidate", len(evs))
	}

	apps := map[string]int{} // app name -> number of servers
	for _, ev := range evs {
		if len(ev.Warnings) > 0 {
			t.Errorf("unexpected warnings: %v", ev.Warnings)
		}
		for _, n := range ev.Nodes {
			if n.Kind == model.KindMCPServer {
				apps[n.Server.App]++
			}
		}
	}
	want := map[string]int{"Claude Desktop": 3, "Claude Code": 2, "Cursor": 3, "VS Code": 2}
	for app, n := range want {
		if apps[app] != n {
			t.Errorf("%s: %d servers, want %d", app, apps[app], n)
		}
	}
}

func TestBrokenConfigBecomesWarning(t *testing.T) {
	home := t.TempDir()
	testutil.CopyFile(t, testutil.Fixture(t, "broken/claude_desktop_config.json"),
		filepath.Join(home, ".config/Claude/claude_desktop_config.json"))
	testutil.CopyFile(t, testutil.Fixture(t, "cursor/mcp.json"), filepath.Join(home, ".cursor/mcp.json"))

	evs := collectAll(t, New(home, "linux"))

	var warnings []string
	servers := 0
	for _, ev := range evs {
		warnings = append(warnings, ev.Warnings...)
		for _, n := range ev.Nodes {
			if n.Kind == model.KindMCPServer {
				servers++
			}
		}
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "parse Claude Desktop config") {
		t.Errorf("warnings = %v", warnings)
	}
	if servers != 3 {
		t.Errorf("cursor servers = %d, want 3 (broken file must not stop the scan)", servers)
	}
}

func TestMissingFileGivesEmptyEvent(t *testing.T) {
	cands := Candidates(t.TempDir(), "linux")
	ev := ScanCandidate(cands[0], "/nowhere")
	if len(ev.Nodes) != 0 || len(ev.Warnings) != 0 || ev.Source == "" {
		t.Errorf("event = %+v", ev)
	}
}

func TestCandidatesPerOS(t *testing.T) {
	for _, goos := range []string{"linux", "darwin"} {
		apps := map[string]bool{}
		for _, c := range Candidates("/h", goos) {
			apps[c.App] = true
			if !strings.HasPrefix(c.Path, "/h/") {
				t.Errorf("%s: path %s not under home", goos, c.Path)
			}
		}
		if len(apps) != 4 {
			t.Errorf("%s: apps = %v, want 4", goos, apps)
		}
	}
	if Candidates("/h", "windows") != nil {
		t.Error("windows is out of scope and should have no candidates")
	}
}
