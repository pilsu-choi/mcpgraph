---
type: Research
title: Codex CLI hooks
description: PreToolUse hook contract of Codex CLI from the official docs, and what could and could not be verified on this machine (codex-cli 0.162.1).
tags: [research, codex, hook, m9]
timestamp: 2026-10-18T08:20:00Z
---

# Codex CLI hooks

Sources: https://developers.openai.com/codex/hooks (redirects to https://learn.chatgpt.com/docs/hooks),
and the input schema `codex-rs/hooks/schema/generated/pre-tool-use.command.input.schema.json` in
github.com/openai/codex.

## From the docs (read 2026-10-18)

- Config: `~/.codex/hooks.json` or `[[hooks.PreToolUse]]` in `~/.codex/config.toml`. Shape
  `{"hooks":{"PreToolUse":[{"matcher":"...","hooks":[{"type":"command","command":"...","timeout":30}]}]}}`.
  `command` is one shell string; there is no `args` field. `timeout` is in seconds (default 600).
- `matcher` is a regex on the tool name; `"*"`, `""` or omitted matches all.
- stdin (schema, no additional properties): `session_id`, `transcript_path` (string or null), `cwd`,
  `hook_event_name` = "PreToolUse", `model`, `permission_mode`, `turn_id`, `tool_name`, `tool_use_id`,
  `tool_input`; optional `agent_id`, `agent_type`. Bash: `tool_input.command` is a string.
  `apply_patch`: `tool_input.command` is the patch text. MCP: `mcp__<server>__<tool>`, `tool_input` =
  the arguments object.
- Output: exit 0 with no output continues. Exit 2 blocks, with the reason on stderr. JSON
  `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"..."}}`
  denies. Plain stdout text is ignored.
- **`ask` is not supported** (nor `decision: approve`, `continue: false`, `stopReason`,
  `suppressOutput`): "Codex marks the hook run as failed, reports the error, and continues the tool
  call." So an unsupported answer fails open; mcpgraph maps ask to deny.
- Trust: "Non-managed hooks must be reviewed and trusted before they run", tied to the hook's hash;
  `/hooks` in the CLI, or `--dangerously-bypass-hook-trust` for one invocation.

## On this machine

- `codex --version`: codex-cli 0.162.1; `codex login status`: logged in with ChatGPT.
- One `codex exec -s read-only --dangerously-bypass-hook-trust` run with a project-local
  `.codex/hooks.json` (matcher `Bash|apply_patch|mcp__.*`, command `mcpgraph hook codex`) in a fresh
  git repo ran `psql -c "DROP TABLE users"` (exit 127, psql missing). **The hook did not run**: no
  audit line was written. Not verified: whether project-local `.codex/hooks.json` is read at all (it
  may need a trusted project, or the user-level file), and whether the bypass flag was enough.
- Codex crash, invalid-output and non-0/2 exit behaviour: not verified; treated as fail open, so the
  hook always answers explicitly (deny JSON, or exit 2 with stderr).
- The Codex payload files in `testdata/hooks/codex` are hand-written from the schema.
