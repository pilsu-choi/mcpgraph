package install

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/collector"
	"github.com/pilsu-choi/mcpgraph/internal/collector/config"
	"github.com/pilsu-choi/mcpgraph/internal/posture"
	"github.com/pilsu-choi/mcpgraph/internal/scan"
	"github.com/pilsu-choi/mcpgraph/internal/testutil"
)

const testBin = "/opt/mcpgraph/bin/mcpgraph"

// fixture is a fake home with every app config, plus the data directory
// install writes to.
type fixture struct {
	home, data string
	orig       map[string][]byte // config path -> original bytes
	modes      map[string]os.FileMode
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{home: testutil.FakeHome(t), orig: map[string][]byte{}, modes: map[string]os.FileMode{}}
	f.data = filepath.Join(f.home, ".local", "share", "mcpgraph")
	// Give one file an unusual mode, to check it survives the round trip.
	if err := os.Chmod(filepath.Join(f.home, ".cursor", "mcp.json"), 0o640); err != nil {
		t.Fatal(err)
	}
	f.snapshot(t)
	return f
}

// snapshot records the current bytes and mode of every config file.
func (f *fixture) snapshot(t *testing.T) {
	t.Helper()
	for _, c := range config.Candidates(f.home, "linux") {
		data, err := os.ReadFile(c.Path)
		if err != nil {
			continue
		}
		info, _ := os.Stat(c.Path)
		f.orig[c.Path] = data
		f.modes[c.Path] = info.Mode().Perm()
	}
}

func (f *fixture) options() Options {
	return Options{Home: f.home, GOOS: "linux", Bin: testBin}
}

func (f *fixture) install(t *testing.T, opts Options) []FileChange {
	t.Helper()
	changes, _, err := Plan(opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(f.data, changes, time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	return changes
}

func (f *fixture) uninstall(t *testing.T) []Step {
	t.Helper()
	steps, err := PlanUninstall(f.data)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyUninstall(f.data, steps); err != nil {
		t.Fatal(err)
	}
	return steps
}

// assertRestored checks every config is byte for byte as recorded, with
// its mode, and that uninstall left no record or backup behind.
func (f *fixture) assertRestored(t *testing.T) {
	t.Helper()
	for path, want := range f.orig {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is not restored byte for byte:\n%s", path, got)
		}
		info, _ := os.Stat(path)
		if info.Mode().Perm() != f.modes[path] {
			t.Errorf("%s mode = %v, want %v", path, info.Mode().Perm(), f.modes[path])
		}
	}
	m, err := LoadManifest(f.data)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Records) != 0 {
		t.Errorf("manifest still has %d records", len(m.Records))
	}
	backups, _ := os.ReadDir(filepath.Join(f.data, Dir, "backups"))
	if len(backups) != 0 {
		t.Errorf("backups left behind: %v", backups)
	}
}

// r4 counts "not proxied" findings in a scan of the fake home.
func r4(t *testing.T, home string) int {
	t.Helper()
	g, err := scan.Run(context.Background(), []collector.Collector{config.New(home, "linux")})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, fd := range posture.Evaluate(g, posture.Default(home)) {
		if fd.RuleID == "R4" {
			n++
		}
	}
	return n
}

// TestRoundTrip is the milestone 8 acceptance test: install, then
// uninstall, restores every file byte for byte, and R4 clears after install.
func TestRoundTrip(t *testing.T) {
	f := newFixture(t)
	if n := r4(t, f.home); n != 6 {
		t.Fatalf("R4 before install = %d, want 6 (the fixture stdio servers)", n)
	}

	changes := f.install(t, f.options())
	if len(changes) != 4 {
		t.Fatalf("changed %d files, want 4", len(changes))
	}
	if n := r4(t, f.home); n != 0 {
		t.Fatalf("R4 after install = %d, want 0", n)
	}
	for path, orig := range f.orig {
		got, _ := os.ReadFile(path)
		if bytes.Equal(got, orig) {
			t.Errorf("%s was not changed by install", path)
		}
		info, _ := os.Stat(path)
		if info.Mode().Perm() != f.modes[path] {
			t.Errorf("install changed the mode of %s", path)
		}
	}

	steps := f.uninstall(t)
	for _, s := range steps {
		if s.Mode != ModeRestore {
			t.Errorf("%s: mode %s, want %s", s.Record.Real, s.Mode, ModeRestore)
		}
	}
	f.assertRestored(t)
	if n := r4(t, f.home); n != 6 {
		t.Fatalf("R4 after uninstall = %d, want 6", n)
	}
}

// TestInstallOnlyTouchesCommandAndArgs checks that everything outside the
// wrapped entries' command and args is unchanged.
func TestInstallOnlyTouchesCommandAndArgs(t *testing.T) {
	f := newFixture(t)
	changes, _, err := Plan(f.options())
	if err != nil {
		t.Fatal(err)
	}
	for _, fc := range changes {
		var before, after map[string]any
		if err := json.Unmarshal(config.StripJSONC(fc.Before), &before); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(config.StripJSONC(fc.After), &after); err != nil {
			t.Fatalf("%s: result is not valid: %v", fc.Real, err)
		}
		dropCommandArgs(before)
		dropCommandArgs(after)
		b, _ := json.Marshal(before)
		a, _ := json.Marshal(after)
		if !bytes.Equal(a, b) {
			t.Errorf("%s: install changed more than command and args:\n%s\n%s", fc.Real, b, a)
		}
		// Comments in the VS Code JSONC file survive.
		if strings.Contains(string(fc.Before), "/*") && !strings.Contains(string(fc.After), "/* remote server") {
			t.Errorf("%s: a comment was lost", fc.Real)
		}
	}
}

// dropCommandArgs deletes every "command" and "args" key, recursively.
func dropCommandArgs(v any) {
	switch t := v.(type) {
	case map[string]any:
		delete(t, "command")
		delete(t, "args")
		for _, x := range t {
			dropCommandArgs(x)
		}
	case []any:
		for _, x := range t {
			dropCommandArgs(x)
		}
	}
}

func TestWrappedCommandLine(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.ProxyFlags = []string{"--policy", "/etc/mcpgraph/policy.yaml"}
	opts.Apps = []string{"Claude Code"}
	f.install(t, opts)

	data, _ := os.ReadFile(filepath.Join(f.home, ".claude.json"))
	root, err := parseJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	entries := findEntries("Claude Code", root, data)
	var slack entry
	for _, e := range entries {
		if e.name == "slack" {
			slack = e
		}
	}
	want := []string{"proxy", "--server", "slack", "--app", "Claude Code", "--policy", "/etc/mcpgraph/policy.yaml",
		"--", "npx", "-y", "slack-mcp-server@1.2.0"}
	if slack.cmd != testBin || strings.Join(slack.argv, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q %q", slack.cmd, slack.argv)
	}
	// The env block (with the secret) is untouched.
	if !bytes.Contains(data, []byte(`"SLACK_BOT_TOKEN": "xoxb-FAKE00000000-FAKEFAKEFAKEFAKE"`)) {
		t.Error("env block changed")
	}
}

func TestFilters(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Servers = []string{"github", "git"}
	changes, skips, err := Plan(opts)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, fc := range changes {
		got = append(got, fc.Servers...)
	}
	if strings.Join(got, ",") != "mcpServers.github,mcpServers.git" {
		t.Fatalf("wrapped %v", got)
	}
	for _, s := range skips {
		if s.Server != "" {
			t.Errorf("filtered-out server reported as skipped: %+v", s)
		}
	}
}

// TestInstallTwice: a second install wraps only the new servers, and one
// uninstall undoes both, newest first, back to the original bytes.
func TestInstallTwice(t *testing.T) {
	f := newFixture(t)
	first := f.options()
	first.Servers = []string{"github"}
	f.install(t, first)

	changes := f.install(t, f.options())
	for _, fc := range changes {
		for _, s := range fc.Servers {
			if s == "mcpServers.github" {
				t.Error("github was wrapped twice")
			}
		}
	}
	m, _ := LoadManifest(f.data)
	if len(m.Records) != 5 { // 1 + 4
		t.Fatalf("records = %d, want 5", len(m.Records))
	}
	f.uninstall(t)
	f.assertRestored(t)
}

// TestUninstallAfterAppChangedFile: the app rewrote its config after
// install. Restoring the backup would lose that change, so uninstall
// removes only the wrapping.
func TestUninstallAfterAppChangedFile(t *testing.T) {
	f := newFixture(t)
	f.install(t, f.options())

	path := filepath.Join(f.home, ".config", "Claude", "claude_desktop_config.json")
	data, _ := os.ReadFile(path)
	changed := bytes.Replace(data, []byte(`"Ctrl+Space"`), []byte(`"Alt+Space"`), 1)
	if err := os.WriteFile(path, changed, 0o600); err != nil {
		t.Fatal(err)
	}

	steps := f.uninstall(t)
	for _, s := range steps {
		want := ModeRestore
		if s.Record.Real == path {
			want = ModeUnwrap
		}
		if s.Mode != want {
			t.Errorf("%s: mode %s, want %s", s.Record.Real, s.Mode, want)
		}
	}

	got, _ := os.ReadFile(path)
	if !bytes.Contains(got, []byte(`"Alt+Space"`)) {
		t.Error("the app's change was lost")
	}
	if bytes.Contains(got, []byte("mcpgraph")) {
		t.Errorf("wrapping not removed:\n%s", got)
	}
	// Same servers as before install.
	var before, after map[string]any
	_ = json.Unmarshal(f.orig[path], &before)
	_ = json.Unmarshal(got, &after)
	b, _ := json.Marshal(before["mcpServers"])
	a, _ := json.Marshal(after["mcpServers"])
	if !bytes.Equal(a, b) {
		t.Errorf("servers differ after unwrap:\n%s\n%s", b, a)
	}
	if n := r4(t, f.home); n != 6 {
		t.Errorf("R4 after uninstall = %d, want 6", n)
	}
}

// TestFileChangedBetweenPlanAndApply: if the app writes the file after
// the plan was made, install refuses instead of overwriting it.
func TestFileChangedBetweenPlanAndApply(t *testing.T) {
	f := newFixture(t)
	opts := f.options()
	opts.Apps = []string{"Cursor"}
	changes, _, err := Plan(opts)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.home, ".cursor", "mcp.json")
	edited := append(append([]byte(nil), f.orig[path]...), ' ')
	if err := os.WriteFile(path, edited, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(f.data, changes, time.Now()); err == nil || !strings.Contains(err.Error(), "changed while") {
		t.Fatalf("err = %v, want a 'changed while' error", err)
	}
	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, edited) {
		t.Error("file was overwritten")
	}
	m, _ := LoadManifest(f.data)
	if len(m.Records) != 0 {
		t.Error("a record was saved for a file that was not written")
	}
}

func TestSymlinkedConfig(t *testing.T) {
	f := newFixture(t)
	path := filepath.Join(f.home, ".cursor", "mcp.json")
	real := filepath.Join(f.home, "dotfiles", "cursor-mcp.json")
	testutil.WriteFile(t, real, f.orig[path])
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, path); err != nil {
		t.Fatal(err)
	}
	opts := f.options()
	opts.Apps = []string{"Cursor"}
	f.install(t, opts)
	if info, _ := os.Lstat(path); info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("install replaced the symlink with a file")
	}
	f.uninstall(t)
	if info, _ := os.Lstat(path); info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("uninstall replaced the symlink with a file")
	}
	if got, _ := os.ReadFile(real); !bytes.Equal(got, f.orig[path]) {
		t.Fatal("symlink target not restored")
	}
}

func TestMissingArgsIsInserted(t *testing.T) {
	src := []byte("{\n  \"mcpServers\": {\n    \"bare\": { \"command\": \"my-server\" }\n  }\n}\n")
	fc, _, err := planFile("Cursor", "/x/mcp.json", src, Options{Bin: testBin})
	if err != nil {
		t.Fatal(err)
	}
	want := `"bare": { "command": "/opt/mcpgraph/bin/mcpgraph", "args": ["proxy", "--server", "bare", "--app", "Cursor", "--", "my-server"] }`
	if !bytes.Contains(fc.After, []byte(want)) {
		t.Fatalf("got\n%s", fc.After)
	}
}

// TestNoSecretInOutputOrManifest: diffs and the manifest never carry a
// raw secret (hard rule 2). Only the backups hold the original bytes.
func TestNoSecretInOutputOrManifest(t *testing.T) {
	f := newFixture(t)
	changes, _, err := Plan(f.options())
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	for _, fc := range changes {
		if err := fc.Diff(&out); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Apply(f.data, changes, time.Now()); err != nil {
		t.Fatal(err)
	}
	// Make one file take the unwrap path, so both uninstall diffs are checked.
	path := filepath.Join(f.home, ".config", "Code", "User", "mcp.json")
	data, _ := os.ReadFile(path)
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	steps, err := PlanUninstall(f.data)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range steps {
		if err := s.Diff(&out); err != nil {
			t.Fatal(err)
		}
	}
	manifest, _ := os.ReadFile(manifestPath(f.data))
	out.Write(manifest)

	for _, secret := range testutil.FakeSecrets {
		if strings.Contains(out.String(), secret) {
			t.Errorf("secret %q leaked into a diff or the manifest", secret)
		}
	}
	if !strings.Contains(out.String(), "****") {
		t.Error("expected masked values in the diff")
	}
}

func TestBackupsArePrivate(t *testing.T) {
	f := newFixture(t)
	f.install(t, f.options())
	m, _ := LoadManifest(f.data)
	for _, r := range m.Records {
		info, err := os.Stat(r.Backup)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("%s mode %v, want 0600", r.Backup, info.Mode().Perm())
		}
		if got, _ := os.ReadFile(r.Backup); !bytes.Equal(got, f.orig[r.Path]) {
			t.Errorf("backup of %s differs from the original", r.Path)
		}
	}
	info, _ := os.Stat(filepath.Dir(m.Records[0].Backup))
	if info.Mode().Perm() != 0o700 {
		t.Errorf("backup dir mode %v, want 0700", info.Mode().Perm())
	}
}

func TestNothingToUninstall(t *testing.T) {
	steps, err := PlanUninstall(t.TempDir())
	if err != nil || len(steps) != 0 {
		t.Fatalf("steps %v, err %v", steps, err)
	}
}

func TestConfigRemovedAfterInstall(t *testing.T) {
	f := newFixture(t)
	f.install(t, f.options())
	gone := filepath.Join(f.home, ".cursor", "mcp.json")
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	delete(f.orig, gone)
	f.uninstall(t)
	f.assertRestored(t)
}
