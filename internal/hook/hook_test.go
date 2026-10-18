package hook

import (
	"reflect"
	"strings"
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/policy"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		adapter Adapter
		in      string
		want    policy.Action
		wantErr string
	}{
		{
			name: "bash", adapter: ClaudeCode,
			in: `{"hook_event_name":"PreToolUse","tool_name":"Bash","cwd":"/p","new_future_field":1,"tool_input":{"command":"ls -la","description":"d"}}`,
			want: policy.Action{Source: model.SourceHook, Agent: "claude-code", App: "Claude Code", Tool: "Bash", Cwd: "/p",
				Text: "ls -la", Args: map[string]any{"command": "ls -la", "description": "d"}},
		},
		{
			name: "read uses the path only", adapter: ClaudeCode,
			in: `{"hook_event_name":"PreToolUse","tool_name":"Write","tool_input":{"file_path":"/p/a.sql","content":"DROP TABLE x"}}`,
			want: policy.Action{Source: model.SourceHook, Agent: "claude-code", App: "Claude Code", Tool: "Write",
				Text: "/p/a.sql", Args: map[string]any{"file_path": "/p/a.sql", "content": "DROP TABLE x"}},
		},
		{
			name: "mcp keeps the full name and no text", adapter: Codex,
			in: `{"hook_event_name":"PreToolUse","tool_name":"mcp__postgres__query","tool_input":{"command":"x","sql":"select 1"}}`,
			want: policy.Action{Source: model.SourceHook, Agent: "codex", App: "Codex", Tool: "mcp__postgres__query",
				Args: map[string]any{"command": "x", "sql": "select 1"}},
		},
		{
			name: "apply_patch", adapter: Codex,
			in: `{"hook_event_name":"PreToolUse","tool_name":"apply_patch","transcript_path":null,"tool_input":{"command":"*** Begin Patch"}}`,
			want: policy.Action{Source: model.SourceHook, Agent: "codex", App: "Codex", Tool: "apply_patch",
				Text: "*** Begin Patch", Args: map[string]any{"command": "*** Begin Patch"}},
		},
		{name: "truncated", adapter: ClaudeCode, in: `{"tool_name":"Bash"`, wantErr: "invalid hook payload"},
		{name: "wrong event", adapter: ClaudeCode, in: `{"hook_event_name":"Stop","tool_name":"Bash"}`, wantErr: "unexpected hook event"},
		{name: "no tool", adapter: Codex, in: `{"hook_event_name":"PreToolUse"}`, wantErr: "no tool_name"},
		{name: "tool_input is a string", adapter: Codex, in: `{"hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":"ls"}`, wantErr: "invalid hook payload"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.adapter.Parse(strings.NewReader(tt.in))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestEncode(t *testing.T) {
	deny := model.Decision{Verdict: model.VerdictDeny, RuleID: "r1", Reason: "no"}
	ask := model.Decision{Verdict: model.VerdictAsk, RuleID: "r2", Reason: "check"}
	tests := []struct {
		name    string
		adapter Adapter
		d       model.Decision
		want    string
	}{
		{"allow prints nothing", ClaudeCode, model.Decision{Verdict: model.VerdictAllow}, ""},
		{"deny", ClaudeCode, deny, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"mcpgraph blocked this by rule r1: no"}}` + "\n"},
		{"ask native", ClaudeCode, ask, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask","permissionDecisionReason":"mcpgraph asks for approval (rule r2): check"}}` + "\n"},
		{"ask on codex is a deny", Codex, ask, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"mcpgraph needs human approval (rule r2): check Do not retry; ask the user to run it themselves or to change that rule in the mcpgraph policy."}}` + "\n"},
		{"unknown verdict fails closed", Codex, model.Decision{Verdict: "maybe", RuleID: "r3"}, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"mcpgraph blocked this by rule r3: "}}` + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(tt.adapter.Encode(tt.d)); got != tt.want {
				t.Errorf("got  %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestLookup(t *testing.T) {
	for name, ok := range map[string]bool{"claude-code": true, "codex": true, "cursor": false, "": false} {
		if _, got := Lookup(name); got != ok {
			t.Errorf("Lookup(%q) = %v", name, got)
		}
	}
}
