---
type: Issue
title: Claude Code hooks fail open by default
description: Mitigated in milestone 9. A crashed, timed-out or malformed PreToolUse hook lets the tool call run unless the handler sets onFailure: block (Claude Code 2.1.295+).
tags: [issue, mitigated, hook, m9, fail-closed]
timestamp: 2026-10-18T08:30:00Z
---

# Claude Code hooks fail open by default

**Status:** mitigated (not closed) by [milestone 9](../milestones/2026-10-18-v2-m9-hook.md).

- Claude Code: `mcpgraph hook claude-code --print-config` emits `"onFailure": "block"`, and the
  hook ([`internal/hook`](../components/2026-10-18-hook.md)) exits 2 on any internal error.
- Codex: no `onFailure` equivalent is documented ([research](../research/2026-10-18-codex-hooks.md)),
  so the hook always answers explicitly (deny JSON or exit 2). Codex end-to-end is still
  [unverified](2026-10-18-codex-hook-unverified.md).
- Remaining risk: a user who writes the hook config by hand without `onFailure`.

According to the [research](../research/external-formats.md) (§B4), a PreToolUse hook that times
out, crashes, exits with a code other than 0 or 2, or prints invalid JSON does **not** block the
tool call. The call continues through the normal permission flow. This is the opposite of the
proxy's hard rule 4 (fail closed).

**Planned mitigations (now implemented, see above):**

- When the hook config is written, set `"onFailure": "block"` on the handler. It exists in Claude
  Code 2.1.295+, and this machine has 2.1.302.
- `mcpgraph hook` itself must print exactly one JSON object on stdout (nothing else) and exit 0, or
  exit 2 with the reason on stderr if anything goes wrong internally. Exit 2 always blocks.
- Use `hookSpecificOutput.permissionDecision` (`allow`/`deny`/`ask`). The top-level
  `decision: approve/block` is deprecated. `ask` is supported natively.

**Open question:** whether a manual hook setup counts as `install` touching configs. Writing to
`~/.claude/settings.json` is an edit outside mcpgraph's data directory, so under hard rule 1 it has
to go through `install` (backup, diff, `--yes`).
