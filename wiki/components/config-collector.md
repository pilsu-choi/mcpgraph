---
type: Go Package
title: internal/collector/config
description: Finds and parses MCP configs of four AI apps and derives credentials and data sources.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/collector/config
tags: [component, go]
timestamp: 2026-10-18T06:50:00Z
---

# `internal/collector/config`

Finds and parses MCP configs of four AI apps and derives credentials and data sources.

Files: `paths.go` (candidates per OS; see [verification issue](../issues/config-path-verification.md)), `claude_desktop.go`, `claude_code.go` (user and per-project scope in `~/.claude.json`), `cursor.go`, `vscode.go` (JSONC via `jsonc.go`), `servers.go` (shared raw shape), `derive.go` (nodes, credentials), `classify.go` (filesystem roots, database URLs, SaaS keywords). Missing files emit empty events, and broken files emit warnings. Why it is shaped this way: [redact at collection](../decisions/redact-at-collection.md).
