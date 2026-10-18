---
type: Milestone
title: m4: Process collector
description: gopsutil-based `Lister`, selection of AI app trees and MCP-like processes, CHILD_OF edges, RUNS_AS matching in the builder, and the `--no-processes` flag.
resource: https://github.com/pilsu-choi/mcpgraph/commit/64f8e58
tags: [milestone, m4]
timestamp: 2026-10-18T06:13:45Z
---

# m4: Process collector

**Commit:** [`64f8e58`](https://github.com/pilsu-choi/mcpgraph/commit/64f8e58)

## Delivered
gopsutil-based `Lister`, selection of AI app trees and MCP-like processes, CHILD_OF edges, RUNS_AS matching in the builder, and the `--no-processes` flag.

## Acceptance check
On the real machine the declared `npx @playwright/mcp@latest` server got RUNS_AS to pid 1654, after fixing two bugs: [npm title](../lessons/npm-process-title.md) and [sh -c](../lessons/shell-c-false-positive.md).

## Go concepts used
- Interface for test doubles (`Lister`)
- Recursion with a depth limit
- Comma-ok idiom for ignoring per-field errors
