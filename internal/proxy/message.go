package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

// message is a parsed JSON-RPC message, kept only for inspection. The
// proxy always forwards the original bytes, never a re-encoding.
//
// Decoding goes through map[string]json.RawMessage on purpose. Decoding
// into a struct would match keys case-insensitively, so
// {"method":"tools/call","Method":"ping"} would read as "ping" here while
// a JavaScript server sees "tools/call": a policy bypass. Map keys are
// exact, and like JSON.parse a repeated key keeps its last value.
type message struct {
	fields map[string]json.RawMessage
}

func parseMessage(line []byte) (message, bool) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(line, &m); err != nil {
		return message{}, false
	}
	return message{fields: m}, true
}

// method returns the "method" string, or "" if absent or not a string.
func (m message) method() string {
	var s string
	if raw, ok := m.fields["method"]; ok && json.Unmarshal(raw, &s) == nil {
		return s
	}
	return ""
}

// id returns the raw "id" value and whether it is present and not null.
// IDs stay raw JSON: 1 and "1" are different IDs and must stay that way.
func (m message) id() (json.RawMessage, bool) {
	raw, ok := m.fields["id"]
	if !ok || string(bytes.TrimSpace(raw)) == "null" {
		return nil, false
	}
	return raw, true
}

// isResponse reports whether the message is a response (result or error,
// no method).
func (m message) isResponse() bool {
	_, hasResult := m.fields["result"]
	_, hasError := m.fields["error"]
	return m.method() == "" && (hasResult || hasError)
}

// idKey turns a raw ID into a map key. Compacting removes insignificant
// whitespace, so `"a"` and ` "a" ` are the same key.
func idKey(raw json.RawMessage) string {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return string(raw)
	}
	return b.String()
}

// toolCall is the part of a tools/call request the policy needs.
type toolCall struct {
	name   string
	args   map[string]any
	modern bool // request carries 2026-07-28 style _meta
}

// modernVersionKey is the per-request protocol version key that MCP
// 2026-07-28 requires in params._meta (research §A3).
const modernVersionKey = "io.modelcontextprotocol/protocolVersion"

// parseToolCall extracts params.name and params.arguments. A tools/call
// whose params cannot be read is an error: the caller denies it (fail
// closed) instead of forwarding something it could not judge.
func parseToolCall(m message) (toolCall, error) {
	var params map[string]json.RawMessage
	if err := json.Unmarshal(m.fields["params"], &params); err != nil {
		return toolCall{}, fmt.Errorf("params is not an object")
	}
	var tc toolCall
	if err := json.Unmarshal(params["name"], &tc.name); err != nil || tc.name == "" {
		return toolCall{}, fmt.Errorf("params.name is missing or not a string")
	}
	if raw, ok := params["arguments"]; ok && string(bytes.TrimSpace(raw)) != "null" {
		if err := json.Unmarshal(raw, &tc.args); err != nil {
			return toolCall{}, fmt.Errorf("params.arguments is not an object")
		}
	}
	var meta map[string]json.RawMessage
	if json.Unmarshal(params["_meta"], &meta) == nil {
		_, tc.modern = meta[modernVersionKey]
	}
	return tc, nil
}

// Keys mcpgraph puts in a denial result's _meta.
// TODO(verify): the research covered where _meta lives, not the naming
// rules for third-party keys; "mcpgraph/" follows the prefix/name shape of
// the spec's own keys.
const (
	metaRuleID  = "mcpgraph/rule_id"
	metaVerdict = "mcpgraph/verdict"
)

// denialResponse builds the reply to a tools/call the proxy refuses. It is
// a normal tool result with isError: true (owner decision, AGENTS.md §15):
// MCP clients SHOULD pass tool execution errors to the model, but only MAY
// pass JSON-RPC errors (research §A4), and the agent must see why.
func denialResponse(id json.RawMessage, d model.Decision, modern bool) []byte {
	result := map[string]any{
		"content": []map[string]string{{"type": "text", "text": denialText(d)}},
		"isError": true,
		"_meta":   map[string]string{metaRuleID: d.RuleID, metaVerdict: string(d.Verdict)},
	}
	if modern {
		// 2026-07-28 results carry resultType; older clients do not know
		// it, and newer ones treat a missing value as "complete".
		result["resultType"] = "complete"
	}
	resp := map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
	b, err := json.Marshal(resp)
	if err != nil {
		// Only possible if id were invalid JSON, which parseMessage rules out.
		b = []byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"mcpgraph: could not encode denial"}}`)
	}
	return append(b, '\n')
}

// denialText is what the model reads.
func denialText(d model.Decision) string {
	if d.Verdict == model.VerdictAsk {
		return fmt.Sprintf("mcpgraph: this tool call needs human approval (policy rule %s) and was not run. %s "+
			"mcpgraph cannot ask the user from inside an MCP connection. To allow it, the user can run the action "+
			"themselves or add an allow rule for it to the policy file given to `mcpgraph proxy --policy`.",
			d.RuleID, d.Reason)
	}
	return fmt.Sprintf("mcpgraph: this tool call was blocked by policy rule %s and was not run. %s", d.RuleID, d.Reason)
}
