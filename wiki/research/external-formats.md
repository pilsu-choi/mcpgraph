---
type: Research
title: External formats: MCP stdio and Claude Code hooks
description: Hard rule 6 check before milestones 7 and 9: MCP stdio wire format (revisions 2026-07-28 and 2025-11-25) and the Claude Code PreToolUse hook contract, each claim with an official source.
tags: [research, mcp, hook, verify, v2]
timestamp: 2026-10-18T07:05:00Z
---

# External formats: MCP stdio and Claude Code hooks

Verified 2026-10-18. Sources are modelcontextprotocol.io and code.claude.com, fetched on that date. Quotes are verbatim excerpts.

> **Main finding:** the latest MCP revision is **`2026-07-28`**, and it changes how the protocol works. It is stateless, with **no `initialize` handshake**. It has **no server-to-client requests**: sampling, elicitation and roots now go through "Multi Round-Trip Requests" (MRTR) as `InputRequiredResult`. Every result has a required `resultType`. The previous ("legacy") revision is `2025-11-25`, and real clients and servers may still speak it. A transparent proxy must handle **both eras**. The stdio framing is identical in both.

---

## A. MCP stdio transport

### A0. Revision
- The latest revision is `2026-07-28`. https://modelcontextprotocol.io/specification/latest/basic/transports resolves to content whose links all point to `/specification/2026-07-28/...`.
- https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning:
  > **Modern**: protocol versions that convey version, identity, and capabilities as per-request metadata (revision `2026-07-28` and later). **Legacy**: protocol versions that establish a session with an `initialize` handshake (`2025-11-25` and earlier).
- Example of a supported-versions list: `"supported": ["2026-07-28", "2025-11-25"]` (same page).

### A1. Framing
- https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/stdio:
  > Each message is a single JSON-RPC request, notification, or response.
  > Messages are delimited by newlines, and **MUST NOT** contain embedded newlines.
  > The server **MUST NOT** write anything to its `stdout` that is not a valid MCP message.
  > The client **MUST NOT** write anything to the server's `stdin` that is not a valid MCP message.
- The 2025-11-25 text is identical (https://modelcontextprotocol.io/specification/2025-11-25/basic/transports#stdio).
- Encoding (https://modelcontextprotocol.io/specification/2026-07-28/basic/transports): "JSON-RPC messages **MUST** be UTF-8 encoded."
- **No Content-Length header framing.** The spec never mentions it. The stdio page says "There is no header layer", and the transports overview says "the stdio binding is just newline-delimited JSON-RPC over a byte stream".
- **Max message size:** the spec does not define one. The proxy must use an unbounded or large buffer, not the 64KB default of Go's `bufio.Scanner`.
- The spec does not say whether a line ends in `\n` or `\r\n`. Accept both on read; write `\n`.

### A2. Batching
- **Not allowed.** Batching was removed in 2025-06-18. https://modelcontextprotocol.io/specification/2025-06-18/changelog:
  > Remove support for JSON-RPC **batching** (PR #416)
- Batching existed only in `2025-03-26`. The current stdio page says "Each message is a single JSON-RPC request, notification, or response".
- A defensive proxy may still see a JSON array from an old `2025-03-26` peer. This is an inference; decide whether to pass arrays through or inspect their elements.

### A3. `tools/call` shape
- https://modelcontextprotocol.io/specification/2026-07-28/server/tools
- Request:
  ```json
  {"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_weather","arguments":{"location":"New York"}}}
  ```
  The page notes:
  > the request examples on this page omit the `_meta` request metadata (`io.modelcontextprotocol/protocolVersion`, `io.modelcontextprotocol/clientInfo`, and `io.modelcontextprotocol/clientCapabilities`). Every request **MUST** include the required `_meta` fields
- `_meta` sits under `params._meta`. Example from https://modelcontextprotocol.io/specification/2026-07-28/basic/index: `"params":{"name":"get_weather","arguments":{...},"_meta":{"traceparent":"..."}}`.
- Required per-request `_meta` keys in 2026-07-28: `io.modelcontextprotocol/protocolVersion` and `io.modelcontextprotocol/clientCapabilities`. `clientInfo` is optional (SHOULD).
- On an MRTR retry, `params` can also contain `inputResponses` and `requestState`. "the JSON-RPC `id` **MUST** be different between the initial request and the retry."
- Successful result:
  ```json
  {"jsonrpc":"2.0","id":2,"result":{"resultType":"complete","content":[{"type":"text","text":"..."}],"isError":false}}
  ```
- `resultType` is new in 2026-07-28. Clients "**MUST** treat an absent `resultType` as `"complete"`" (basic/index). A result can also be `"resultType":"input_required"`, which is not final: the client retries the call with a new id.

### A4. JSON-RPC error vs `isError: true`
- https://modelcontextprotocol.io/specification/2026-07-28/server/tools#error-handling
  > **Protocol Errors** indicate issues with the request structure itself that models are less likely to be able to fix: Unknown tool / Malformed requests / Server errors … returned as standard JSON-RPC errors (example `"code": -32602, "message": "Unknown tool: invalid_tool_name"`)
  > **Tool Execution Errors** contain actionable feedback that language models can use to self-correct … reported in tool results with `isError: true`
  > Clients **MAY** provide protocol errors to language models, though these are less likely to result in successful recovery. Clients **SHOULD** provide tool execution errors to language models to enable self-correction.
- **Conclusion:** if the LLM should reliably see a deny message, return a **result with `isError: true`** and a text content item, plus `"resultType":"complete"` for modern-era requests. A JSON-RPC error is only "MAY" forwarded to the model. The spec does not cover policy refusals by a proxy at all, so this is a design choice based on the guidance above.
- Error codes (https://modelcontextprotocol.io/specification/2026-07-28/basic/index#error-codes):
  > MCP uses the standard JSON-RPC 2.0 error codes (`-32700`, `-32600` to `-32603`) for general protocol failures.
  > `-32000` to `-32019` — legacy … new implementations **SHOULD NOT** use codes from this sub-range at all.
  > `-32020` to `-32099` — reserved for the MCP specification … Implementations **MUST NOT** emit any code from this sub-range that is not defined by this specification
  > New error codes for purposes not defined by this specification **SHOULD** be allocated outside the JSON-RPC reserved range (`-32768` to `-32000`)
- MCP-defined codes: `-32020` HeaderMismatch, `-32021` MissingRequiredClientCapability, `-32022` UnsupportedProtocolVersion.
- **Implication:** if mcpgraph uses a JSON-RPC error for a policy denial, it should use a custom code **outside -32768..-32000**, or the generic `-32603`/`-32600`. It must **not** invent a code in -32000..-32099.

### A5. IDs and message direction
- IDs (basic/index, both revisions):
  > Requests **MUST** include a string or integer ID. Unlike base JSON-RPC, the ID **MUST NOT** be `null`.
  - Error responses may omit `id` "where the ID could not be read due a malformed request".
  - Keep IDs as raw JSON (`json.RawMessage`) so `1` and `"1"` stay distinct and are never re-encoded.
- **Modern (2026-07-28)**, from https://modelcontextprotocol.io/specification/2026-07-28/basic/transports:
  > No other message direction exists … servers do not initiate JSON-RPC requests and clients do not send JSON-RPC responses.
  - The stdio page adds: "The server **MUST NOT** write JSON-RPC *requests* to `stdout`. Server-to-client interactions are carried in `InputRequiredResult` replies".
  - The server can still send notifications to the client, such as `notifications/progress`, `notifications/message`, and subscription notifications.
- **Legacy (2025-11-25)**, from https://modelcontextprotocol.io/specification/2025-11-25/basic:
  > Requests are sent from the client to the server or vice versa
  - In this era the server sends requests such as `sampling/createMessage`, `elicitation/create`, `roots/list` and `ping`, and the client answers them.
- **Implication:** the proxy must pass requests and responses in both directions, and match each response to its request per direction, because IDs from the two sides can collide.

### A6. Cancellation and progress
- https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/cancellation:
  ```json
  {"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":"123","reason":"User requested cancellation"}}
  ```
  > stdio: … The client **MUST** send a `notifications/cancelled` notification referencing the request ID.
  > The client **SHOULD** ignore any response to the cancelled request that arrives afterward
  - In 2026-07-28, servers send `notifications/cancelled` only to tear down `subscriptions/listen`.
  - **Implication:** if mcpgraph denies a call locally and the client later cancels that id, the proxy should drop the cancel rather than forward it to the server, which never saw the id.
- https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/progress:
  - The request opts in with `params._meta.progressToken` (a string or integer).
  - The server then sends:
    ```json
    {"jsonrpc":"2.0","method":"notifications/progress","params":{"progressToken":"abc123","progress":50,"total":100,"message":"..."}}
    ```

### A7. stderr
- stdio page (same in both revisions):
  > The server **MAY** write UTF-8 strings to `stderr` for any logging purposes including informational, debug, and error messages.
  > The client **MAY** capture, forward, or ignore the server's `stderr` output and **SHOULD NOT** assume `stderr` output indicates error conditions.
- **Implication:** the proxy should pass the child's stderr through and keep its own logs on stderr. It must never write logs to stdout.

### A8. Handshake and version
- **Legacy (2025-11-25)**, https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle:
  - `initialize` request with `params.protocolVersion: "2025-11-25"`, `capabilities` and `clientInfo`.
  - The result carries `protocolVersion`, `capabilities`, `serverInfo` and `instructions`.
  - Then the client sends `{"jsonrpc":"2.0","method":"notifications/initialized"}`.
- **Modern (2026-07-28)**, https://modelcontextprotocol.io/specification/2026-07-28/changelog:
  > Make MCP stateless: remove the `initialize`/`notifications/initialized` handshake. Every request now carries its protocol version and client capabilities in `_meta`
  - New `server/discover`, which servers MUST implement.
  - Unsupported version returns error `-32022` with `data.supported`/`data.requested`.
  - Removed: `ping`, `logging/setLevel` and `notifications/roots/list_changed`.
- stdio backward compatibility (stdio page): a dual-era client probes with `server/discover`. On "any other error, or does not respond within a reasonable timeout: the server is legacy. Fall back to the `initialize` handshake." A transparent proxy should forward this probe unchanged.
- Shutdown (stdio): the client closes stdin, waits, then SIGTERM, then SIGKILL. The proxy should close the child's stdin when its own stdin hits EOF.

---

## B. Claude Code hooks (PreToolUse)

Source for all of B: https://code.claude.com/docs/en/hooks

### B1. Config locations and shape
- Locations:
  > `~/.claude/settings.json` | All your projects
  > `.claude/settings.json` | Single project | Yes, can be committed to the repo
  > `.claude/settings.local.json` | Single project | No, gitignored
- Managed policy, plugin `hooks/hooks.json`, and skill or agent frontmatter can also define hooks.
- Hooks merge across levels:
  > All matching hooks run in parallel. If you define the same handler in more than one settings file, it runs once.
- Shape:
  ```json
  {"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"...","args":[],"timeout":600}]}]}}
  ```
- Matcher syntax:
  > `"*"`, `""`, or omitted | Match all
  > Only letters, digits, `_`, `-`, spaces, `,`, and `\|` | Exact string, or list of exact strings separated by `\|` or `,`
  > Contains any other character | JavaScript regular expression, unanchored
  - MCP tools: "MCP tools follow the naming pattern `mcp__<server>__<tool>`". The `.*` is required: "a matcher like `mcp__memory` … is compared as an exact string and matches no tool". Use `mcp__memory__.*`.
- Optional `if` field, using permission-rule syntax such as `"Bash(git *)"`. It holds exactly one rule.
- Command handler fields:
  - `command`
  - `args`: exec form, no shell
  - `shell` (`bash` or `powershell`)
  - `async`
  - `timeout` (seconds)
  - `statusMessage`
  - `onFailure` (`"continue"`/`"block"`, v2.1.295+)

### B2. stdin payload (PreToolUse)
- Common fields:
  - `session_id`, `transcript_path`, `cwd` and `hook_event_name`.
  - `permission_mode` is one of `"default"`, `"plan"`, `"acceptEdits"`, `"auto"`, `"dontAsk"`, `"bypassPermissions"`.
  - Also documented: `prompt_id`, `scratchpad_dir`, `effort`, `agent_id` and `agent_type`, which are not always present.
- Example:
  ```json
  {"session_id":"abc123","transcript_path":"/home/user/.claude/projects/.../transcript.jsonl","cwd":"/home/user/my-project","permission_mode":"default","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"npm test","description":"Run test suite","timeout":120000,"run_in_background":false},"tool_use_id":"toolu_01ABC123..."}
  ```
- Bash `tool_input` fields: `command`, `description`, `timeout` (ms) and `run_in_background`.
- MCP tools: `tool_name` = `mcp__<server>__<tool>`. Additionally:
  > the input also carries `mcp_server`, an object with the server's `name` and a `source` … Base trust decisions on `source` rather than on `name` or the `mcp__<server>__` tool-name prefix. The `mcp_server` field requires Claude Code v2.1.274 or later.
  - `source` values include `plugin`, `sdk`, `user` and `project`.
  - **Parse unknown fields leniently**: the payload grows over time.

### B3. Output contract
- Exit codes:
  > Exit 0 means success, and is the intended exit code when you print JSON for structured control.
  > Exit with code 2 to block the action … The message that comes with the block is your hook's stderr.
  > Exit code 1 without valid JSON output is a non-blocking error … If your hook is meant to enforce a policy, use `exit 2`.
  - Exit-code-2 table: `PreToolUse | Yes | Blocks the tool call`.
  - "A hook that blocks by exiting 2 routes the same way as `"deny"`: Claude sees the stderr message as the denial reason."
  - Exit 2 overrides JSON: "Even a `permissionDecision` of `"allow"` doesn't let the action through."
- JSON parsing: stdout is parsed as JSON "when the whole output is one JSON object with nothing around it but whitespace". Print nothing else on stdout. "Claude never sees stderr from a hook that exits 0."
- PreToolUse decision: the decision goes inside `hookSpecificOutput`, which has four outcomes: allow, deny, ask, defer.
  ```json
  {"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Database writes are not allowed"}}
  ```
  > `"allow"` skips the permission prompt, except for actions no mode auto-approves … `"deny"` prevents the tool call. `"ask"` prompts the user to confirm. `"defer"` exits gracefully so the tool can be resumed later. Deny and ask rules are still evaluated regardless of what the hook returns
- **Who sees the reason:**
  > For `"ask"`, shown to the user in the permission prompt. When Claude Code denies the call in a `-p` run where no one can answer that prompt, Claude reads the reason in the tool result instead. For `"deny"`, shown to Claude. For `"allow"` and `"defer"`, written to the debug log only
- **`ask` is supported natively.** The prompt labels its origin as `[settings]`, `[plugin:<name>]` or `[skill]`.
- Several hooks: "precedence is `deny` > `defer` > `ask` > `allow`."
- **Deprecated form:**
  > PreToolUse previously used top-level `decision` and `reason` fields, but these are deprecated for this event. … The deprecated values `"approve"` and `"block"` map to `"allow"` and `"deny"` respectively.
  - Emit only `hookSpecificOutput.permissionDecision`.

### B4. Timeout and failure
- Default timeout:
  > Defaults: 600 for `command`, `http`, and `mcp_tool`
  - That is 600 seconds. It is lowered only for UserPromptSubmit, PreModelSwitch, PostModelSwitch and MessageDisplay, not PreToolUse.
- **Fails open by default:**
  > A timed-out `command`, `http`, or `mcp_tool` hook doesn't block the tool call, except for hooks whose failure blocks the action. The call continues through the normal permission flow, so don't count on a stalled hook to act as a gate.
  > On most events, when a hook fails or times out, Claude Code still carries out the action.
- To fail closed, set `"onFailure":"block"` on the handler (v2.1.295+). It then blocks the call on any of these failures:
  - the command can't start
  - an exit code other than 0 or 2
  - a timeout
  - invalid or schema-failing JSON
- Invalid JSON with exit 0 and no `onFailure` is a non-blocking error, so the call proceeds.

### B5. Other fields
- `continue` (default `true`): "If `false`, Claude stops processing entirely after the hook runs. Takes precedence over any event-specific decision fields".
- `stopReason`: shown to the user when `continue` is false.
- `suppressOutput`: "Has no effect: Claude Code accepts the field but doesn't act on it."
- `systemMessage`: "Warning message shown to the user."
- `terminalSequence`: an escape sequence the hook asks Claude Code to emit.
- `hookSpecificOutput.updatedInput`: "Replaces the entire input object, so include unchanged fields alongside modified ones." Pair it with allow or ask; it is ignored for defer.
- `hookSpecificOutput.additionalContext`: a string added to Claude's context.
- The output cap is 10,000 chars, covering `additionalContext`, `systemMessage` and plain stdout.
- Separate event `PermissionRequest`: it uses a `decision` object with `behavior` `"allow"`/`"deny"`. That is a different contract; don't confuse it with PreToolUse.

### B6. Local machine (read-only check)
- `claude --version` printed `2.1.302 (Claude Code)`. That is at least 2.1.295 (`onFailure`) and at least 2.1.274 (`mcp_server` input).
- `~/.claude/settings.json` top-level keys: `model`, `enabledPlugins`, `tui`, `theme`, `autoMode`, `modelSettings`. **There is no `hooks` key.**

---

## Open questions / unverified
1. **Which MCP revision real clients speak.** Claude Desktop, Cursor, Claude Code and VS Code could each send `initialize` (legacy) or `server/discover` plus `_meta` (modern). This is not on official spec pages; capture real traffic per client. Design the proxy to be era-agnostic.
2. **No max message size** is defined, and the spec does not say whether `\r\n` is allowed.
3. **Whether Claude Code or Claude Desktop shows JSON-RPC error messages to the model.** The spec only says "MAY". Use `isError: true` for denials to be safe, and test this empirically.
4. **No spec guidance on proxies/intermediaries refusing calls.** The `isError` result is our design choice, based on the spec's tool-execution-error guidance.
5. **`defer`** semantics ("resumed later") were not explored in detail. They are likely irrelevant for mcpgraph.
6. **The exact form of the `-p` denial text** for `ask` in non-interactive runs was not explored further.
7. **Hook stdin size limits and encoding** are not documented. Assume UTF-8 JSON.
8. The hooks page is about 270K chars. Sections past about 200K chars (PermissionRequest details, debug) were not fully read.
