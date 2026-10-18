package main

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/audit"
	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/testutil"
)

// TestProxyCommand runs `mcpgraph proxy -- cat`. cat echoes every line it
// gets, so whatever reaches "the server" comes straight back: the ping
// comes back, the DROP TABLE call does not, and a denial arrives instead.
func TestProxyCommand(t *testing.T) {
	if _, err := exec.LookPath("cat"); err != nil {
		t.Skip("cat not available")
	}
	dir := t.TempDir()
	ping := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	drop := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"query","arguments":{"sql":"DROP TABLE users"}}}`

	cmd := newRootCmd()
	var out, errOut bytes.Buffer
	cmd.SetIn(strings.NewReader(ping + "\n" + drop + "\n"))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"proxy", "--data-dir", dir, "--server", "postgres", "--app", "Cursor", "--", "cat"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("%v (stderr %q)", err, errOut.String())
	}

	got := out.String()
	if !strings.Contains(got, ping+"\n") {
		t.Errorf("ping was not passed through unchanged: %q", got)
	}
	if strings.Contains(got, "DROP TABLE users") || !strings.Contains(got, `"isError":true`) || !strings.Contains(got, "block-destructive-sql") {
		t.Errorf("DROP TABLE must be denied, not forwarded: %q", got)
	}

	res, err := audit.ReadFile(filepath.Join(dir, audit.FileName), audit.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Decisions) != 1 {
		t.Fatalf("audit has %d decisions, want 1", len(res.Decisions))
	}
	d := res.Decisions[0]
	if d.Verdict != model.VerdictDeny || d.Agent != "postgres" || d.App != "Cursor" || d.Tool != "query" {
		t.Errorf("audited %+v", d)
	}
}

// TestProxyBadPolicyFailsClosed: a broken policy file must not stop the
// proxy (other traffic still flows) but every tool call is denied.
func TestProxyBadPolicyFailsClosed(t *testing.T) {
	if _, err := exec.LookPath("cat"); err != nil {
		t.Skip("cat not available")
	}
	pol := filepath.Join(t.TempDir(), "bad.yaml")
	testutil.WriteFile(t, pol, []byte("version: 1\nrules:\n  - id: x\n    verdikt: allow\n"))
	call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query","arguments":{"sql":"SELECT 1"}}}`

	cmd := newRootCmd()
	var out, errOut bytes.Buffer
	cmd.SetIn(strings.NewReader(call + "\n"))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"proxy", "--data-dir", t.TempDir(), "--policy", pol, "--", "cat"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "SELECT 1") || !strings.Contains(out.String(), "mcpgraph-fail-closed") {
		t.Errorf("with a broken policy the call must be denied: %q", out.String())
	}
}

func TestProxyNeedsACommand(t *testing.T) {
	if _, err := run(t, "proxy"); err == nil {
		t.Fatal("want an error without a server command")
	}
}
