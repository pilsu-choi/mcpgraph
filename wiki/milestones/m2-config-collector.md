---
type: Milestone
title: m2: Config collector
description: Parsers for Claude Desktop, Claude Code (user and project scope), Cursor and VS Code (JSONC), candidate paths per OS, derivation of Credential and DataSource nodes, fixtures, and the inventory table.
resource: https://github.com/pilsu-choi/mcpgraph/commit/3626db6
tags: [milestone, m2]
timestamp: 2026-10-18T06:10:36Z
---

# m2: Config collector

**Commit:** [`3626db6`](https://github.com/pilsu-choi/mcpgraph/commit/3626db6)

## Delivered
Parsers for Claude Desktop, Claude Code (user and project scope), Cursor and VS Code (JSONC), candidate paths per OS, derivation of Credential and DataSource nodes, fixtures, and the inventory table.

## Acceptance check
`mcpgraph scan` on the owner's machine lists Claude Code (`~/.claude.json`, 0 servers). Parser tests pass from fixtures alone (fake `$HOME`).

## Go concepts used
- Implicit interface satisfaction (`parser`)
- Sorting map keys because map iteration order is random
- `json.RawMessage` for lazy decoding
- `select` with `ctx.Done()` on channel send
- Unexported type to keep raw secrets in one package
