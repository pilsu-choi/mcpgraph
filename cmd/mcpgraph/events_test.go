package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/audit"
	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// writeAudit creates a data dir with an audit log of the given decisions.
func writeAudit(t *testing.T, ds ...model.Decision) string {
	t.Helper()
	dir := t.TempDir()
	l, err := audit.Open(filepath.Join(dir, audit.FileName))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	for _, d := range ds {
		if err := l.Append(d); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func sampleDecisions() []model.Decision {
	now := time.Now().UTC()
	return []model.Decision{
		{Time: now.Add(-48 * time.Hour), Source: model.SourceHook, Agent: "claude-code", App: "Claude Code", Tool: "Bash",
			Summary: "go test ./...", Verdict: model.VerdictAllow},
		{Time: now.Add(-time.Hour), Source: model.SourceHook, Agent: "claude-code", App: "Claude Code", Tool: "Bash",
			Summary: `psql -c "DROP TABLE users"`, RuleID: "block-destructive-sql", Verdict: model.VerdictDeny},
		{Time: now.Add(-time.Minute), Source: model.SourceProxy, Agent: "postgres", App: "Claude Desktop", Tool: "query",
			Summary: "DELETE FROM users", RuleID: "block-destructive-sql", Verdict: model.VerdictDeny},
	}
}

func TestEventsListsAndFilters(t *testing.T) {
	dir := writeAudit(t, sampleDecisions()...)
	tests := []struct {
		args    []string
		want    []string
		notWant []string
	}{
		{nil, []string{"DECISIONS (3)", "go test", "DROP TABLE users", "postgres (Claude Desktop)"}, nil},
		{[]string{"--verdict", "deny"}, []string{"DECISIONS (2)", "DENY"}, []string{"go test"}},
		{[]string{"--since", "24h"}, []string{"DECISIONS (2)"}, []string{"go test"}},
		{[]string{"--since", "3d"}, []string{"DECISIONS (3)"}, nil},
		{[]string{"--source", "proxy", "--tool", "QUERY"}, []string{"DECISIONS (1)", "DELETE FROM users"}, nil},
		{[]string{"--rule", "block-destructive-sql", "-n", "1"}, []string{"DECISIONS (1)", "DELETE FROM users"}, []string{"DROP"}},
		{[]string{"-f", "jsonl", "--verdict", "allow"}, []string{`"verdict":"allow"`, `"summary":"go test ./..."`}, []string{"DECISIONS"}},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			out, err := run(t, append([]string{"events", "--data-dir", dir}, tt.args...)...)
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("output missing %q:\n%s", w, out)
				}
			}
			for _, w := range tt.notWant {
				if strings.Contains(out, w) {
					t.Errorf("output should not contain %q:\n%s", w, out)
				}
			}
		})
	}
}

func TestEventsRejectsBadFlags(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		{"--verdict", "block"}, {"--source", "daemon"}, {"--since", "yesterday"}, {"-f", "xml"},
	} {
		if _, err := run(t, append([]string{"events", "--data-dir", dir}, args...)...); err == nil {
			t.Errorf("%v: want an error", args)
		}
	}
}

func TestEventsEmptyLog(t *testing.T) {
	out, err := run(t, "events", "--data-dir", t.TempDir())
	if err != nil || !strings.Contains(out, "DECISIONS (0)") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestScanShowsRecentDecisions(t *testing.T) {
	dir := writeAudit(t, sampleDecisions()...)
	out, _ := run(t, "scan", "--data-dir", dir)
	for _, w := range []string{"RECENT DECISIONS (3)", "Claude Desktop / postgres", "block-destructive-sql"} {
		if !strings.Contains(out, w) {
			t.Errorf("scan output missing %q:\n%s", w, out)
		}
	}
	out, _ = run(t, "scan", "--data-dir", dir, "--decisions", "0")
	if strings.Contains(out, "RECENT DECISIONS") {
		t.Error("--decisions 0 should leave decisions out")
	}
}

func TestParseSince(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		in   string
		want time.Time
		err  bool
	}{
		{"", time.Time{}, false},
		{"90m", now.Add(-90 * time.Minute), false},
		{"7d", now.AddDate(0, 0, -7), false},
		{"2026-10-01T00:00:00Z", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), false},
		{"1.5d", time.Time{}, true},
		{"soon", time.Time{}, true},
	}
	for _, tt := range tests {
		got, err := parseSince(tt.in, now)
		if (err != nil) != tt.err || !got.Equal(tt.want) {
			t.Errorf("parseSince(%q) = %v, %v", tt.in, got, err)
		}
	}
}
