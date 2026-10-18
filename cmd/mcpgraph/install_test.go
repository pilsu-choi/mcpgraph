package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/testutil"
)

// runHome is run without the fixture home that run adds, for tests that
// pass their own --home and need it to stay the same across commands.
func runHome(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}

// configFiles reads every fixture config under a fake home.
func configFiles(t *testing.T, home string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, rel := range []string{".config/Claude/claude_desktop_config.json", ".claude.json", ".cursor/mcp.json", ".config/Code/User/mcp.json"} {
		data, err := os.ReadFile(filepath.Join(home, rel))
		if err != nil {
			t.Fatal(err)
		}
		out[rel] = data
	}
	return out
}

func TestInstallCommandRoundTrip(t *testing.T) {
	home := testutil.FakeHome(t)
	orig := configFiles(t, home)
	bin := "/usr/local/bin/mcpgraph"

	// Dry run: a diff, nothing written.
	out, err := runHome(t, "--home", home, "install", "--bin", bin)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Dry run") || !strings.Contains(out, `+      "command": "/usr/local/bin/mcpgraph",`) {
		t.Fatalf("dry run output:\n%s", out)
	}
	for _, secret := range testutil.FakeSecrets {
		if strings.Contains(out, secret) {
			t.Errorf("secret %q printed", secret)
		}
	}
	for rel, data := range configFiles(t, home) {
		if !bytes.Equal(data, orig[rel]) {
			t.Fatalf("dry run changed %s", rel)
		}
	}

	if _, err := runHome(t, "--home", home, "install", "--bin", bin, "--yes"); err != nil {
		t.Fatal(err)
	}
	report, _ := runHome(t, "--home", home, "--no-processes", "report")
	if strings.Contains(report, "R4") {
		t.Errorf("R4 still reported after install:\n%s", report)
	}

	out, err = runHome(t, "--home", home, "uninstall", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "byte for byte") {
		t.Errorf("uninstall output:\n%s", out)
	}
	for rel, data := range configFiles(t, home) {
		if !bytes.Equal(data, orig[rel]) {
			t.Errorf("%s not restored byte for byte", rel)
		}
	}
}

// TestInstallRefusesTestBinary: the test binary is not named mcpgraph, so
// without --bin install must refuse rather than write a path scans would
// not recognise.
func TestInstallRefusesTestBinary(t *testing.T) {
	home := testutil.FakeHome(t)
	_, err := runHome(t, "--home", home, "install", "--yes")
	if err == nil || !strings.Contains(err.Error(), "--bin") {
		t.Fatalf("err = %v, want a hint about --bin", err)
	}
}

func TestUninstallWithoutInstall(t *testing.T) {
	out, err := runHome(t, "--home", t.TempDir(), "uninstall")
	if err != nil || !strings.Contains(out, "Nothing to uninstall") {
		t.Fatalf("out %q, err %v", out, err)
	}
}
