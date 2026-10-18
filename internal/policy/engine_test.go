package policy

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/testutil"
)

func TestParseRejectsBadPolicies(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string // substring of the error
	}{
		{"unknown key", "version: 1\nrules:\n  - id: a\n    verdikt: deny\n    match: {sql: [truncate]}\n", "verdikt"},
		{"bad version", "version: 2\nrules: []\n", "version"},
		{"bad verdict", "version: 1\nrules:\n  - id: a\n    verdict: block\n    match: {sql: [truncate]}\n", "unknown verdict"},
		{"bad default", "version: 1\ndefault: maybe\n", "default"},
		{"missing id", "version: 1\nrules:\n  - verdict: deny\n    match: {sql: [truncate]}\n", "missing id"},
		{"duplicate id", "version: 1\nrules:\n  - {id: a, verdict: deny, match: {sql: [truncate]}}\n  - {id: a, verdict: ask, match: {sql: [truncate]}}\n", "duplicate"},
		{"no matcher", "version: 1\nrules:\n  - {id: a, verdict: deny}\n", "no matcher"},
		{"bad sql class", "version: 1\nrules:\n  - {id: a, verdict: deny, match: {sql: [drop_everything]}}\n", "sql class"},
		{"bad regex", "version: 1\nrules:\n  - {id: a, verdict: deny, match: {command: ['(']}}\n", "match.command"},
		{"bad glob", "version: 1\nrules:\n  - {id: a, verdict: deny, match: {paths: ['[']}}\n", "match.paths"},
		{"bad source", "version: 1\nrules:\n  - {id: a, verdict: deny, when: {sources: [daemon]}, match: {sql: [truncate]}}\n", "source"},
		{"bad delete mode", "version: 1\nrules:\n  - {id: a, verdict: ask, match: {recursive_delete: everywhere}}\n", "recursive_delete"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse([]byte(tt.yaml))
			if err == nil {
				_, err = New(p, Options{})
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func mustEngine(t *testing.T, yaml string) *RuleEngine {
	t.Helper()
	p, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	e, err := New(p, Options{Home: testHome, Now: func() time.Time { return time.Date(2026, 10, 18, 7, 0, 0, 0, time.FixedZone("KST", 9*3600)) }})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestFirstMatchWins(t *testing.T) {
	e := mustEngine(t, `
version: 1
rules:
  - {id: first, verdict: ask, match: {text: ['drop']}}
  - {id: second, verdict: deny, match: {sql: [drop_table]}}
`)
	d, err := e.Decide(context.Background(), Action{Source: model.SourceHook, Text: "DROP TABLE t"})
	if err != nil {
		t.Fatal(err)
	}
	if d.RuleID != "first" || d.Verdict != model.VerdictAsk {
		t.Fatalf("got %s/%s, want first/ask", d.RuleID, d.Verdict)
	}
	if !d.Time.Equal(time.Date(2026, 10, 18, 7, 0, 0, 0, time.FixedZone("KST", 9*3600))) || d.Time.Location() != time.UTC {
		t.Errorf("time = %v, want the injected time in UTC", d.Time)
	}
}

func TestDefaultVerdictAndFilters(t *testing.T) {
	e := mustEngine(t, `
version: 1
default: deny
rules:
  - id: allow-read-tools
    verdict: allow
    when: {sources: [proxy], tools: ['read_.*', 'list']}
    match: {text: ['.*']}
`)
	tests := []struct {
		a    Action
		rule string
		want model.Verdict
	}{
		{Action{Source: model.SourceProxy, Tool: "read_file", Text: "x"}, "allow-read-tools", model.VerdictAllow},
		{Action{Source: model.SourceProxy, Tool: "LIST", Text: "x"}, "allow-read-tools", model.VerdictAllow},
		{Action{Source: model.SourceProxy, Tool: "listen", Text: "x"}, "", model.VerdictDeny}, // tools are anchored
		{Action{Source: model.SourceHook, Tool: "read_file", Text: "x"}, "", model.VerdictDeny},
	}
	for _, tt := range tests {
		d, err := e.Decide(context.Background(), tt.a)
		if err != nil {
			t.Fatal(err)
		}
		if d.RuleID != tt.rule || d.Verdict != tt.want {
			t.Errorf("%s/%s: got %q/%s, want %q/%s", tt.a.Source, tt.a.Tool, d.RuleID, d.Verdict, tt.rule, tt.want)
		}
	}
}

func TestDecideCancelledContext(t *testing.T) {
	e := defaultEngine(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.Decide(ctx, bash("ls")); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

// fakeEngine lets tests make Decide fail, panic or hang.
type fakeEngine struct {
	err   error
	panic bool
	block chan struct{}
}

func (f fakeEngine) Decide(ctx context.Context, a Action) (model.Decision, error) {
	if f.panic {
		panic("boom")
	}
	if f.block != nil {
		<-f.block // ignores ctx on purpose: Judge must not depend on it
	}
	return model.Decision{Verdict: model.VerdictAllow}, f.err
}

func TestJudgeFailsClosed(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	tests := []struct {
		name string
		e    Engine
		want string
	}{
		{"error", fakeEngine{err: errors.New("disk on fire")}, "disk on fire"},
		{"panic", fakeEngine{panic: true}, "panic"},
		{"hang", fakeEngine{block: block}, "deadline exceeded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			d := Judge(context.Background(), tt.e, bash(`psql -c "SELECT 1"`), 50*time.Millisecond)
			if time.Since(start) > time.Second {
				t.Errorf("Judge took %v; it must return soon after the timeout", time.Since(start))
			}
			if d.Verdict != model.VerdictDeny || d.RuleID != FailClosedRuleID || !strings.Contains(d.Reason, tt.want) {
				t.Fatalf("got %+v, want a fail-closed denial mentioning %q", d, tt.want)
			}
		})
	}
}

func TestJudgePassesDecisionThrough(t *testing.T) {
	d := Judge(context.Background(), defaultEngine(t), bash(`psql -c "DROP TABLE users"`), time.Second)
	if d.Verdict != model.VerdictDeny || d.RuleID != "block-destructive-sql" {
		t.Fatalf("got %s/%s", d.RuleID, d.Verdict)
	}
	if !strings.Contains(d.Summary, "DROP TABLE users") || d.Agent != "claude-code" || d.Tool != "Bash" || d.Source != model.SourceHook {
		t.Errorf("decision fields not filled: %+v", d)
	}
}

// TestDecisionNeverContainsSecrets is hard rule 2 for the policy engine:
// the Decision (which goes to the audit log and the graph) must not carry
// raw secrets from the command or the tool arguments.
func TestDecisionNeverContainsSecrets(t *testing.T) {
	e := defaultEngine(t)
	actions := []Action{
		bash(`PGPASSWORD=FakePassw0rdXyz psql -h db -c "DROP TABLE users"`),
		bash(`psql postgres://app:FakePassw0rdXyz@db/prod -c "DELETE FROM users"`),
		bash(`curl -H "Authorization: Bearer Zx9Qw8Er7Ty6Ui5Op4As3Df2FAKE" https://api.example.com`),
		bash(`gh auth login --with-token ghp_FAKEa1b2c3d4e5f6g7h8i9j0FAKEFAKE1234 && cat ~/.ssh/id_rsa`),
		{Source: model.SourceProxy, Agent: "slack", Tool: "post", Args: map[string]any{
			"token": "xoxb-FAKE00000000-FAKEFAKEFAKEFAKE", "text": "hi",
			"nested": map[string]any{"key": "sk-ant-FAKE0123456789abcdefFAKE"},
		}},
		{Source: model.SourceProxy, Agent: "x", Tool: "fetch", Args: map[string]any{"url": "https://h/x?api_key=FAKEqueryKey1234567890abcd"}},
	}
	for _, a := range actions {
		for _, d := range []model.Decision{mustDecide(t, e, a), Judge(context.Background(), fakeEngine{err: errors.New("x")}, a, time.Second)} {
			raw, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range testutil.FakeSecrets {
				if strings.Contains(string(raw), s) {
					t.Errorf("decision %s contains raw secret %q", raw, s)
				}
			}
		}
	}
}

func mustDecide(t *testing.T, e Engine, a Action) model.Decision {
	t.Helper()
	d, err := e.Decide(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestSummaryIsCapped(t *testing.T) {
	s := Summary(strings.Repeat("a ", 500))
	if n := len([]rune(s)); n > maxSummary+1 {
		t.Fatalf("summary has %d runes, want at most %d", n, maxSummary+1)
	}
}
