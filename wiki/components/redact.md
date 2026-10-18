---
type: Go Package
title: internal/redact
description: Secret detection and masking.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/redact
tags: [component, go]
timestamp: 2026-10-18T07:15:00Z
---

> **v2 addition:** `Text(s)` redacts free text (shell commands, SQL). It covers `NAME=value` assignments, the word after Bearer/Basic/token/password, quoted `--flag=value`, and everything `Args` already masks. The policy engine uses it for `Decision.Summary`.

# `internal/redact`

Secret detection and masking.

`Detect(key, value)` checks, in order: references (`${...}`, `$NAME`) → not secret; `Bearer x` → secret; known prefixes (ghp_, sk-ant-, xoxb-, AKIA, ...); secret-named keys (TOKEN, PASSWORD, API_KEY, ...) unless the value is a path or bool; high entropy (≥20 chars, letters+digits, Shannon ≥3.5, not a path, version or UUID). `Args` handles `--flag=value` and `--api-key VALUE`. `URL` masks passwords and secret query values. `Mask` → `****` + last 4.
