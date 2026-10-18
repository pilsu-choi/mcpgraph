---
type: Index
title: Components
description: One page per Go package.
tags: [component]
timestamp: 2026-10-18T07:15:00Z
---

# Components

| Package | Role |
| --- | --- |
| [`internal/model`](model.md) | Shared types: Node, Edge, Graph, Event, Finding, NodeID. |
| [`internal/collector/config`](config-collector.md) | Finds and parses MCP configs of four AI apps and derives credentials and data sources. |
| [`internal/collector/process`](process-collector.md) | Selects AI app processes, their descendants and MCP-like processes; builds CHILD_OF edges. |
| [`internal/graph`](graph-builder.md) | Single-writer builder, RUNS_AS linking and snapshot diff. |
| [`internal/redact`](redact.md) | Secret detection and masking. |
| [`internal/posture`](posture.md) | Posture rule interface, Default set (R1–R7), Evaluate and HasHigh. |
| [`internal/policy`](policy.md) | Runtime policy engine: YAML, matchers, SQL classifier, Decide, fail-closed Judge. |
| [`internal/report`](report.md) | Inventory and findings tables, JSON, Mermaid. |
| [`internal/sink`](sink.md) | Interface for forwarding snapshots to a central server; only a no-op exists. |
| [`tools/termsvg`](termsvg.md) | Renders terminal output as an SVG terminal window for README captures. |

Command: `cmd/mcpgraph`, one cobra binary. `scan`, `report` and `export` work. `proxy`, `hook`,
`install`, `uninstall` and `events` are stubs that name their milestone.

## Archived (branch `archive/v1-daemon`)

| Package | Role |
| --- | --- |
| [`internal/collector/watch`](watcher.md) | fsnotify watcher that re-parses changed config files after a debounce. |
| [`internal/store`](store.md) | SQLite snapshots with changes, pruning to the last 500. |
| [`internal/daemon`](daemon.md) | Long-running pipeline: initial full scan, watcher, periodic re-scan, publish. |
| [`internal/api`](api.md) | Read-only JSON API on 127.0.0.1: /graph, /findings, /snapshots. |
| `cmd/mcpgraphd` | Daemon binary. |
