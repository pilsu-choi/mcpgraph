package hook

import (
	"encoding/json"
	"strings"
)

// ConfigSnippet returns the JSON to merge into the agent's hook config
// (~/.claude/settings.json or ~/.codex/hooks.json). The matcher is a
// regex, so the ".*" in "mcp__.*" matches every MCP tool (a bare "mcp__"
// would match none). It only builds text:
// writing app configs is reserved for `mcpgraph install` (hard rule 1).
func (a Adapter) ConfigSnippet(bin string) ([]byte, error) {
	var handler map[string]any
	if a.NativeAsk {
		// Claude Code: exec form (no shell), and "onFailure":"block" so a
		// crashed or timed-out hook blocks the call instead of failing open.
		handler = map[string]any{
			"type": "command", "command": bin, "args": []string{"hook", a.Agent},
			"timeout": 10, "onFailure": "block",
		}
	} else {
		// Codex: "command" is one shell string (no args field), and its
		// docs list no failure setting.
		// TODO(verify): Codex crash and invalid-output behaviour.
		handler = map[string]any{
			"type": "command", "command": shellQuote(bin) + " hook " + a.Agent, "timeout": 10,
		}
	}
	cfg := map[string]any{"hooks": map[string]any{"PreToolUse": []any{
		map[string]any{"matcher": a.Matcher, "hooks": []any{handler}},
	}}}
	return json.MarshalIndent(cfg, "", "  ")
}

// shellQuote wraps s in single quotes for a POSIX shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
