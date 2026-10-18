---
type: Go Package
title: internal/audit
description: Append-only JSONL log of runtime decisions, filters for mcpgraph events, and the collector that turns decisions into Decision graph nodes.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/audit
tags: [component, go, audit, v2, m6]
timestamp: 2026-10-18T07:25:00Z
---

# `internal/audit`

| File | Role |
| --- | --- |
| `audit.go` | `DefaultDataDir`, `Open` (dir 0700, file 0600, `O_APPEND`), `Log.Append` (redacts again, one `Write` per line, mutex in-process), `Read`/`ReadFile` (Scanner raised to 1 MiB, malformed lines skipped and counted, missing file = empty) |
| `filter.go` | `Filter{Since, Verdict, Source, Agent, Tool, RuleID, Limit}` for `mcpgraph events` |
| `collector.go` | `Collector{Path, Limit}`: newest N decisions → `Decision` nodes (Source `audit`); `DecisionNode` with a stable ID |

**Where it writes.** `~/.local/share/mcpgraph/audit.jsonl` (or `$XDG_DATA_HOME/mcpgraph`), or
`~/Library/Application Support/mcpgraph` on macOS. `--data-dir` overrides it. This is mcpgraph's
own data directory, so it does not conflict with hard rule 1. With `--home` set, `$XDG_DATA_HOME` is
ignored, so a fake home never reads the real log.

**Many writers.** Each hook call is a separate process, and there is one proxy per server. `O_APPEND`
makes the kernel move to the end of the file on every `write`, and one decision is one `write`, so
concurrent lines do not overwrite each other. A test runs four goroutines on two file handles,
writing 800 lines, and finds every line intact.

**Linking.** The collector does not draw edges, because it cannot see the server and app nodes
from other collectors. The [graph builder](graph-builder.md) adds `DECIDED_ON` in `linkDecisions`:

- a proxy decision links to the MCP server with `Name == Agent`, limited to `App` when the
  decision has one;
- a hook decision links to the AIApp with `Name == App`;
- a decision with no target stays in the graph without an edge.
