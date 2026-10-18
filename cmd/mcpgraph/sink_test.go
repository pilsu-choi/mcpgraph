package main

import (
	"bytes"
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/central"
	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/sink"
)

// runIn runs the CLI with an explicit data dir.
func runIn(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append(args, "--data-dir", dir))
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}

func TestSinkSetStatusOff(t *testing.T) {
	dir := t.TempDir()
	if out, err := runIn(t, dir, "sink", "status"); err != nil || !strings.Contains(out, "off") {
		t.Fatalf("status before set = %q, %v", out, err)
	}
	out, err := runIn(t, dir, "sink", "set", "http://collector.example.com:7600", "--token", "super-secret-token", "--host", "laptop")
	if err != nil || !strings.Contains(out, "warning") {
		t.Fatalf("set = %q, %v (want a plain-http warning)", out, err)
	}
	st, err := os.Stat(filepath.Join(dir, sink.ConfigFile))
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("sink.json: %v, %v", st, err)
	}
	out, err = runIn(t, dir, "sink", "status")
	if err != nil || strings.Contains(out, "super-secret-token") || !strings.Contains(out, "laptop") || !strings.Contains(out, "not shipped yet: 0") {
		t.Fatalf("status = %q, %v", out, err)
	}
	if _, err := runIn(t, dir, "sink", "off"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, sink.ConfigFile)); !os.IsNotExist(err) {
		t.Fatalf("sink.json still there: %v", err)
	}
	if _, err := runIn(t, dir, "sink", "set", "ftp://nope"); err == nil {
		t.Fatal("want an error for a non-http URL")
	}
	if _, err := runIn(t, dir, "sink", "flush"); err == nil {
		t.Fatal("flush with the sink off should say so")
	}
}

func TestEventsFromCollector(t *testing.T) {
	store, err := central.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	srv := httptest.NewServer(central.Handler(store, "tok"))
	defer srv.Close()
	ev := []sink.Event{{ID: "1", Decision: model.Decision{Time: time.Now(), Source: "proxy", Tool: "query", Summary: "drop", Verdict: model.VerdictDeny}}}
	if _, err := store.Add("laptop", ev, time.Now()); err != nil {
		t.Fatal(err)
	}
	out, err := runIn(t, t.TempDir(), "events", "--collector", srv.URL, "--token", "tok", "--host", "laptop")
	if err != nil || !strings.Contains(out, "DENY") {
		t.Fatalf("events = %q, %v", out, err)
	}
	if _, err := runIn(t, t.TempDir(), "events", "--collector", srv.URL, "--token", "wrong"); err == nil {
		t.Fatal("want an error for a wrong token")
	}
}
