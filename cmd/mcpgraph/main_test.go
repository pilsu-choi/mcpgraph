package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/testutil"
)

// run executes the CLI in-process with a fake home and no process scan.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append(args, "--home", testutil.FakeHome(t), "--no-processes"))
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}

func TestScanExitsTwoOnHighFinding(t *testing.T) {
	out, err := run(t, "scan")
	var e exitError
	if !errors.As(err, &e) || e.code != 2 {
		t.Fatalf("err = %v, want exit code 2", err)
	}
	for _, want := range []string{"AI APPS (4)", "MCP SERVERS (10)", "HIGH (", "[R1]", "[R3]", "[R4]"} {
		if !strings.Contains(out, want) {
			t.Errorf("scan output missing %q", want)
		}
	}
}

func TestReportCleanHomeExitsZero(t *testing.T) {
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"report", "--format", "json", "--home", t.TempDir(), "--no-processes"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if strings.TrimSpace(out.String()) != "[]" {
		t.Errorf("output = %q, want []", out.String())
	}
}

func TestExportFormats(t *testing.T) {
	out, err := run(t, "export", "--format", "mermaid")
	if err != nil || !strings.HasPrefix(out, "flowchart LR") {
		t.Fatalf("mermaid export: err=%v out=%.40q", err, out)
	}
	if _, err := run(t, "export", "--format", "dot"); err == nil {
		t.Fatal("unknown format should fail")
	}
}

func TestNoSecretsInCLIOutput(t *testing.T) {
	for _, args := range [][]string{{"scan"}, {"report"}, {"report", "-f", "json"}, {"export", "-f", "json"}, {"export", "-f", "mermaid"}} {
		out, _ := run(t, args...)
		for _, s := range testutil.FakeSecrets {
			if strings.Contains(out, s) {
				t.Errorf("%v output leaks %q", args, s)
			}
		}
	}
}
