---
type: Issue
title: Codex hook is not verified end to end
description: A real codex exec ran a shell command without invoking the project-local hook; it is unknown whether project-local hooks need a trusted project or ~/.codex/hooks.json.
tags: [issue, open, hook, codex, m9]
timestamp: 2026-10-18T08:30:00Z
---

# Codex hook is not verified end to end

**Status:** open.

A real `codex exec` (Codex CLI 0.162.1, `-s read-only --dangerously-bypass-hook-trust`, with a
project-local `.codex/hooks.json`) ran the command **without invoking the hook**. The adapter is
tested against payloads built from the official docs
([research](../research/2026-10-18-codex-hooks.md)), but Codex has not been seen calling it.

**Unknown:** whether project-local hooks need a trusted project, or whether only
`~/.codex/hooks.json` is read.

**Next step:** repeat the run with `~/.codex/hooks.json` in a throwaway `CODEX_HOME`, so the real
home is untouched.

Related: [milestone 9](../milestones/2026-10-18-v2-m9-hook.md),
[`internal/hook`](../components/2026-10-18-hook.md), [hooks fail open](hook-fails-open.md).
