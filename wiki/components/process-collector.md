---
type: Go Package
title: internal/collector/process
description: Selects AI app processes, their descendants and MCP-like processes; builds CHILD_OF edges.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/collector/process
tags: [component, go]
timestamp: 2026-10-18T06:45:00Z
---

# `internal/collector/process`

Selects AI app processes, their descendants and MCP-like processes; builds CHILD_OF edges.

`Lister` interface with a gopsutil implementation. `AppName` recognises Claude Desktop/Code, Cursor and VS Code (including `.vscode-server`). `mcpLike` checks non-flag args for "mcp", ignores tools such as grep, and for `sh -c SCRIPT` looks only at the first word ([lesson](../lessons/shell-c-false-positive.md)). Single-string argv is split ([lesson](../lessons/npm-process-title.md)). It skips its own pid.
