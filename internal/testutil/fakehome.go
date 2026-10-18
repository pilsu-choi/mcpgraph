// Package testutil holds helpers shared by tests in several packages.
package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// FakeSecrets are the raw secret values planted in testdata/configs. Tests
// assert that none of them ever appears in output.
var FakeSecrets = []string{
	"ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKEFAKE1234",
	"FakePassw0rdXyz",
	"xoxb-FAKE00000000-FAKEFAKEFAKEFAKE",
	"FAKEqueryKey1234567890abcd",
	"sk-ant-FAKE0123456789abcdefFAKE",
	"Zx9Qw8Er7Ty6Ui5Op4As3Df2FAKE",
}

// RepoRoot returns the repository root, found relative to this source file
// so tests work from any package directory.
func RepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// Fixture returns the path of a file under testdata/configs.
func Fixture(t *testing.T, rel string) string {
	t.Helper()
	return filepath.Join(RepoRoot(t), "testdata", "configs", rel)
}

// FakeHome creates a temporary home directory with every fixture copied to
// the place the Linux candidate paths expect. Tests use it instead of the
// real $HOME so they never read the developer's machine.
func FakeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	copies := map[string]string{
		"claude_desktop/claude_desktop_config.json": ".config/Claude/claude_desktop_config.json",
		"claude_code/.claude.json":                  ".claude.json",
		"cursor/mcp.json":                           ".cursor/mcp.json",
		"vscode/mcp.json":                           ".config/Code/User/mcp.json",
	}
	for src, dst := range copies {
		CopyFile(t, Fixture(t, src), filepath.Join(home, dst))
	}
	return home
}

// CopyFile copies src to dst, creating parent directories.
func CopyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	WriteFile(t, dst, data)
}

// WriteFile writes data to path, creating parent directories.
func WriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
