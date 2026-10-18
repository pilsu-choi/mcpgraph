// Package hook adapts coding-agent PreToolUse hooks to the policy engine.
//
// A coding agent runs `mcpgraph hook <agent>` once per tool call and writes
// one JSON payload to its stdin. Parse turns the payload into a
// policy.Action, the same type the MCP proxy uses, so one policy covers both
// interception paths. Encode turns the Decision back into the agent's
// answer.
package hook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/pilsu-choi/mcpgraph/internal/model"
	"github.com/pilsu-choi/mcpgraph/internal/policy"
)

// Adapter knows one coding agent. Claude Code and Codex send the same
// fields mcpgraph needs, so they differ only in names and in whether the
// agent can ask the user.
type Adapter struct {
	Agent string // Action.Agent, also the CLI argument
	App   string // Action.App, the AI app's display name
	// NativeAsk is true when the agent supports permissionDecision "ask".
	// Codex does not: it marks the hook failed and runs the tool anyway
	// (fail open), so an ask verdict must be sent as a deny.
	NativeAsk bool
	// Matcher is the tool-name regex for the agent's hook config.
	Matcher string
}

// Adapters are the supported coding agents.
var (
	ClaudeCode = Adapter{Agent: "claude-code", App: "Claude Code", NativeAsk: true, Matcher: "Bash|Read|Edit|Write|mcp__.*"}
	Codex      = Adapter{Agent: "codex", App: "Codex", Matcher: "Bash|apply_patch|mcp__.*"}
)

// Lookup finds an adapter by its CLI name.
func Lookup(agent string) (Adapter, bool) {
	for _, a := range []Adapter{ClaudeCode, Codex} {
		if a.Agent == agent {
			return a, true
		}
	}
	return Adapter{}, false
}

// maxPayload bounds how much of stdin Parse reads. A truncated payload is
// invalid JSON, so an oversized one is blocked, not half-judged.
const maxPayload = 8 << 20

// payload holds the PreToolUse fields mcpgraph uses. encoding/json ignores
// every other field, which is what we want: agents add fields over time.
type payload struct {
	HookEventName string         `json:"hook_event_name"`
	ToolName      string         `json:"tool_name"`
	ToolInput     map[string]any `json:"tool_input"`
	Cwd           string         `json:"cwd"`
}

// Parse reads one PreToolUse payload and returns the action it describes.
// The error never contains payload text (it may hold secrets).
func (a Adapter) Parse(r io.Reader) (policy.Action, error) {
	var p payload
	// A Decoder reads one JSON value from a stream; unlike json.Unmarshal
	// it does not need the whole input in memory first and tolerates
	// trailing data.
	if err := json.NewDecoder(io.LimitReader(r, maxPayload)).Decode(&p); err != nil {
		return policy.Action{}, fmt.Errorf("invalid hook payload: %w", err)
	}
	if p.HookEventName != "PreToolUse" {
		return policy.Action{}, fmt.Errorf("unexpected hook event %q (want PreToolUse)", safe(p.HookEventName))
	}
	if p.ToolName == "" {
		return policy.Action{}, errors.New("hook payload has no tool_name")
	}
	return policy.Action{
		Source: model.SourceHook,
		Agent:  a.Agent,
		App:    a.App,
		Tool:   p.ToolName,
		Args:   p.ToolInput,
		Text:   actionText(p),
		Cwd:    p.Cwd,
	}, nil
}

// actionText picks the text the matchers inspect. Shell-like tools (Bash,
// Codex's apply_patch) carry it in tool_input.command. A file tool (Read,
// Edit, Write) contributes only its path, so a file whose content mentions
// DROP TABLE is not mistaken for a query. MCP tools return "" and the
// engine then inspects all their string arguments.
func actionText(p payload) string {
	if strings.HasPrefix(p.ToolName, "mcp__") {
		return ""
	}
	for _, key := range []string{"command", "file_path"} {
		if s, ok := p.ToolInput[key].(string); ok {
			return s
		}
	}
	return ""
}

// safe keeps a value we echo in an error short and on one line.
func safe(s string) string {
	if len(s) > 40 {
		s = s[:40]
	}
	return strings.ReplaceAll(s, "\n", " ")
}

type output struct {
	Hook hookOutput `json:"hookSpecificOutput"`
}

type hookOutput struct {
	EventName string `json:"hookEventName"`
	Decision  string `json:"permissionDecision"`
	Reason    string `json:"permissionDecisionReason"`
}

// Encode returns what the hook prints on stdout, or nil for an allow.
//
// Allow prints nothing on purpose: permissionDecision "allow" would skip
// the agent's own permission prompt, and mcpgraph only adds restrictions.
func (a Adapter) Encode(d model.Decision) []byte {
	var decision string
	switch d.Verdict {
	case model.VerdictAllow:
		return nil
	case model.VerdictAsk:
		decision = "ask"
		if !a.NativeAsk {
			decision = "deny"
		}
	default: // deny, and anything unknown: fail closed
		decision = "deny"
	}
	b, err := json.Marshal(output{hookOutput{"PreToolUse", decision, a.reason(d, decision)}})
	if err != nil { // cannot happen: only strings
		return nil
	}
	return append(b, '\n')
}

// reason is shown to the model (deny) or the user (ask). It names the rule
// and carries only the policy's own text, never the tool input.
func (a Adapter) reason(d model.Decision, decision string) string {
	id := d.RuleID
	if id == "" {
		id = "default-policy"
	}
	switch {
	case d.Verdict != model.VerdictAsk:
		return fmt.Sprintf("mcpgraph blocked this by rule %s: %s", id, d.Reason)
	case decision == "ask":
		return fmt.Sprintf("mcpgraph asks for approval (rule %s): %s", id, d.Reason)
	default: // an ask the agent cannot show, sent as a deny
		return fmt.Sprintf("mcpgraph needs human approval (rule %s): %s Do not retry; ask the user to run it "+
			"themselves or to change that rule in the mcpgraph policy.", id, d.Reason)
	}
}
