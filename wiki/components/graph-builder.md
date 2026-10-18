---
type: Go Package
title: internal/graph
description: Single-writer builder, RUNS_AS linking and snapshot diff.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/graph
tags: [component, go]
timestamp: 2026-10-18T06:45:00Z
---

# `internal/graph`

Single-writer builder, RUNS_AS linking and snapshot diff.

`Builder.Apply` replaces per source. `Graph()` merges sorted sources, dedupes nodes, drops dangling edges, links processes, then sorts. `Run` batches already-buffered events with a non-blocking `select`. `Matches(server, process)` requires every non-flag server arg to appear in the process args. When two apps declare the same server, `preferOwningApp` picks the app the process descends from. `Diff` compares nodes by ID through their JSON form and ignores `Host.ScannedAt`. See [single writer](../decisions/single-writer-builder.md).
