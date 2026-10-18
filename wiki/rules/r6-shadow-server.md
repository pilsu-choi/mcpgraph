---
type: Risk Rule
title: R6: Undeclared MCP-like process
description: A process flagged MCP-like with no RUNS_AS edge on itself or any ancestor. Only the top of an MCP-like chain is reported.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/rules/r6_shadow_server.go
tags: [rule, r6, medium]
timestamp: 2026-10-18T06:18:47Z
---

# R6: Undeclared MCP-like process

- **Severity:** medium
- **Detects:** A process flagged MCP-like with no RUNS_AS edge on itself or any ancestor. Only the top of an MCP-like chain is reported.
- **Rationale (shown in the finding):** Its tools and permissions are invisible to anyone reviewing configs.
- **Test cases:** undeclared chain → 1 (top only); declared server and its children → 0; not MCP-like → 0.

Source: [`internal/rules/r6_shadow_server.go`](https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/rules/r6_shadow_server.go). Rule interface and ordering: [rules component](../components/rules.md).
