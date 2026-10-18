package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pilsu-choi/mcpgraph/internal/audit"
	"github.com/pilsu-choi/mcpgraph/internal/hook"
	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/policy"
	"github.com/pilsu-choi/mcpgraph/internal/testutil"
)

// Reasons of the default policy, as the hook words them.
const (
	sqlDeny     = `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"mcpgraph blocked this by rule block-destructive-sql: Destructive SQL is blocked: DROP TABLE, DROP DATABASE/SCHEMA, TRUNCATE and DELETE without a limiting WHERE clause (WHERE 1=1 does not count)."}}` + "\n"
	secretDeny  = `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"mcpgraph blocked this by rule block-secret-read: Access to secret files is blocked: SSH keys (~/.ssh), AWS credentials and .env files."}}` + "\n"
	rmAsk       = `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask","permissionDecisionReason":"mcpgraph asks for approval (rule ask-recursive-delete): Recursive delete (rm -r) of a path outside the current project directory needs approval."}}` + "\n"
	rmCodexDeny = `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"mcpgraph needs human approval (rule ask-recursive-delete): Recursive delete (rm -r) of a path outside the current project directory needs approval. Do not retry; ask the user to run it themselves or to change that rule in the mcpgraph policy."}}` + "\n"
)

// runHookCmd runs `mcpgraph hook` with the payload file as stdin and
// returns stdout, stderr, the exit code and the audit decisions.
func runHookCmd(t *testing.T, stdin string, args ...string) (stdout, stderr string, code int, ds []model.Decision) {
	t.Helper()
	dir := t.TempDir()
	cmd := newRootCmd()
	var out, errOut bytes.Buffer
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(append([]string{"hook", "--home", "/home/testuser", "--data-dir", dir}, args...))
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		code = exitCode(err)
	}
	res, err := audit.ReadFile(filepath.Join(dir, audit.FileName), audit.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	return out.String(), errOut.String(), code, res.Decisions
}

func TestHookCommand(t *testing.T) {
	tests := []struct {
		agent, file string
		stdout      string
		verdict     model.Verdict
		rule        string
	}{
		{"claude-code", "bash-drop-table", sqlDeny, model.VerdictDeny, "block-destructive-sql"},
		{"claude-code", "bash-ls", "", model.VerdictAllow, ""},
		{"claude-code", "bash-rm-rf-home", rmAsk, model.VerdictAsk, "ask-recursive-delete"},
		{"claude-code", "read-ssh-key", secretDeny, model.VerdictDeny, "block-secret-read"},
		{"claude-code", "mcp-delete", sqlDeny, model.VerdictDeny, "block-destructive-sql"},
		{"claude-code", "bash-secret-drop-table", sqlDeny, model.VerdictDeny, "block-destructive-sql"},
		{"claude-code", "captured-bash-echo", "", model.VerdictAllow, ""},
		{"codex", "bash-drop-table", sqlDeny, model.VerdictDeny, "block-destructive-sql"},
		{"codex", "bash-ls", "", model.VerdictAllow, ""},
		{"codex", "bash-rm-rf-home", rmCodexDeny, model.VerdictAsk, "ask-recursive-delete"},
		{"codex", "apply-patch-env", secretDeny, model.VerdictDeny, "block-secret-read"},
		{"codex", "mcp-delete", sqlDeny, model.VerdictDeny, "block-destructive-sql"},
		{"codex", "bash-secret-drop-table", sqlDeny, model.VerdictDeny, "block-destructive-sql"},
	}
	for _, tt := range tests {
		t.Run(tt.agent+"/"+tt.file, func(t *testing.T) {
			payload, err := os.ReadFile(filepath.Join(testutil.RepoRoot(t), "testdata", "hooks", tt.agent, tt.file+".json"))
			if err != nil {
				t.Fatal(err)
			}
			stdout, stderr, code, ds := runHookCmd(t, string(payload), tt.agent)
			if code != 0 || stdout != tt.stdout {
				t.Errorf("exit %d, stdout %q (stderr %q), want exit 0, stdout %q", code, stdout, stderr, tt.stdout)
			}
			if len(ds) != 1 {
				t.Fatalf("audit has %d decisions, want 1", len(ds))
			}
			wantApp := map[string]string{"claude-code": "Claude Code", "codex": "Codex"}[tt.agent]
			d := ds[0]
			if d.Verdict != tt.verdict || d.RuleID != tt.rule || d.Source != model.SourceHook || d.Agent != tt.agent || d.App != wantApp {
				t.Errorf("audited %+v", d)
			}
			assertNoSecrets(t, stdout, stderr, auditText(t, ds))
		})
	}
}

func auditText(t *testing.T, ds []model.Decision) string {
	t.Helper()
	var sb strings.Builder
	for _, d := range ds {
		sb.WriteString(d.Summary + d.Reason)
	}
	return sb.String()
}

func assertNoSecrets(t *testing.T, texts ...string) {
	t.Helper()
	for _, text := range texts {
		for _, s := range testutil.FakeSecrets {
			if strings.Contains(text, s) {
				t.Errorf("secret %q leaked into %q", s, text)
			}
		}
	}
}

// TestHookSummaryIsRedacted: the secret in the command must be masked in
// the audit summary, not only absent from the answer.
func TestHookSummaryIsRedacted(t *testing.T) {
	payload := `{"hook_event_name":"PreToolUse","tool_name":"Bash","cwd":"/p","tool_input":{"command":"PGPASSWORD=FakePassw0rdXyz psql -c \"DROP TABLE users\""}}`
	_, _, _, ds := runHookCmd(t, payload, "claude-code")
	if len(ds) != 1 || strings.Contains(ds[0].Summary, "FakePassw0rdXyz") || !strings.Contains(ds[0].Summary, "PGPASSWORD") {
		t.Errorf("summary %+v", ds)
	}
}

// TestHookFailClosed: every failure blocks with exit 2 and a reason on
// stderr that names the fail-closed rule and contains no payload text.
func TestHookFailClosed(t *testing.T) {
	good := `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"ls"}}`
	badPolicy := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(badPolicy, []byte("rules: [unclosed"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, stdin string
		args        []string
	}{
		{"malformed json", `{"tool_name":"Bash","tool_input":{"command":"ls FakePassw0rdXyz"`, []string{"claude-code"}},
		{"empty stdin", "", []string{"codex"}},
		{"not an object", `["FakePassw0rdXyz"]`, []string{"claude-code"}},
		{"wrong event", `{"hook_event_name":"PostToolUse","tool_name":"Bash"}`, []string{"claude-code"}},
		{"no tool name", `{"hook_event_name":"PreToolUse"}`, []string{"codex"}},
		{"unknown agent", good, []string{"cursor"}},
		{"bad policy file", good, []string{"claude-code", "--policy", badPolicy}},
		{"missing policy file", good, []string{"codex", "--policy", "/nonexistent/policy.yaml"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code, ds := runHookCmd(t, tt.stdin, tt.args...)
			if code != 2 || stdout != "" || !strings.Contains(stderr, "mcpgraph-fail-closed") {
				t.Errorf("exit %d, stdout %q, stderr %q; want exit 2, empty stdout, rule named", code, stdout, stderr)
			}
			if len(ds) != 0 {
				t.Errorf("audited %+v", ds)
			}
			assertNoSecrets(t, stdout, stderr)
		})
	}
}

// hangingEngine never answers until its context ends.
type hangingEngine struct{}

func (hangingEngine) Decide(ctx context.Context, _ policy.Action) (model.Decision, error) {
	<-ctx.Done()
	return model.Decision{}, ctx.Err()
}

// TestHookEngineTimeoutDenies: a stuck engine yields a deny that names the
// fail-closed rule and is audited.
func TestHookEngineTimeoutDenies(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetContext(context.Background()) // runHook is called without Execute
	var out, errOut bytes.Buffer
	cmd.SetIn(strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"ls"}}`))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	dir := t.TempDir()
	opts := &options{dataDir: dir}
	start := time.Now()
	if err := runHook(cmd, opts, hook.ClaudeCode, hangingEngine{}, 50*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 2*time.Second {
		t.Error("hook waited for the stuck engine")
	}
	if !strings.Contains(out.String(), `"permissionDecision":"deny"`) || !strings.Contains(out.String(), "mcpgraph-fail-closed") {
		t.Errorf("stdout %q", out.String())
	}
	res, err := audit.ReadFile(filepath.Join(dir, audit.FileName), audit.Filter{})
	if err != nil || len(res.Decisions) != 1 || res.Decisions[0].RuleID != "mcpgraph-fail-closed" {
		t.Errorf("audit %+v, %v", res, err)
	}
}

// TestHookAuditUnavailable: if the audit log cannot be opened the hook
// still answers.
func TestHookAuditUnavailable(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newRootCmd()
	var out, errOut bytes.Buffer
	cmd.SetIn(strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"DROP TABLE users"}}`))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	// The data directory is a regular file, so the log cannot be created.
	cmd.SetArgs([]string{"hook", "claude-code", "--data-dir", file})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if out.String() != sqlDeny {
		t.Errorf("stdout %q", out.String())
	}
}

func TestHookPrintConfig(t *testing.T) {
	tests := []struct {
		agent string
		want  []string
	}{
		{"claude-code", []string{`"onFailure": "block"`, `"command": "/opt/mcpgraph"`, `"claude-code"`, `Bash|Read|Edit|Write|mcp__.*`}},
		{"codex", []string{`"command": "'/opt/mcpgraph' hook codex"`, `Bash|apply_patch|mcp__.*`}},
	}
	for _, tt := range tests {
		t.Run(tt.agent, func(t *testing.T) {
			stdout, _, code, ds := runHookCmd(t, "", tt.agent, "--print-config", "--bin", "/opt/mcpgraph")
			if code != 0 || len(ds) != 0 {
				t.Fatalf("exit %d, audit %v", code, ds)
			}
			for _, w := range tt.want {
				if !strings.Contains(stdout, w) {
					t.Errorf("config lacks %q:\n%s", w, stdout)
				}
			}
		})
	}
}
