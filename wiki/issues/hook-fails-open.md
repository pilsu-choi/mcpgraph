---
type: Issue
title: Claude Code hooks fail open by default
description: A crashed, timed-out or malformed PreToolUse hook lets the tool call run unless the handler sets onFailure: block (Claude Code 2.1.295+).
tags: [issue, open, hook, m9, fail-closed]
timestamp: 2026-10-18T07:05:00Z
---

# Claude Code hooks fail open by default

**Status:** open, for [milestone 9](../milestones/index.md).

According to the [research](../research/external-formats.md) (§B4), a PreToolUse hook that times
out, crashes, exits with a code other than 0 or 2, or prints invalid JSON does **not** block the
tool call. The call continues through the normal permission flow. This is the opposite of the
proxy's hard rule 4 (fail closed).

**Mitigations for m9:**

- When the hook config is written, set `"onFailure": "block"` on the handler. It exists in Claude
  Code 2.1.295+, and this machine has 2.1.302.
- `mcpgraph hook` itself must print exactly one JSON object on stdout (nothing else) and exit 0, or
  exit 2 with the reason on stderr if anything goes wrong internally. Exit 2 always blocks.
- Use `hookSpecificOutput.permissionDecision` (`allow`/`deny`/`ask`). The top-level
  `decision: approve/block` is deprecated. `ask` is supported natively.

**Open question:** whether a manual hook setup counts as `install` touching configs. Writing to
`~/.claude/settings.json` is an edit outside mcpgraph's data directory, so under hard rule 1 it has
to go through `install` (backup, diff, `--yes`).
