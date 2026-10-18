---
type: Decision
title: Archive the v1 daemon, store and API
description: The daemon, SQLite store, localhost API and fsnotify watcher moved to the archive/v1-daemon branch until stretch milestone 12.
resource: https://github.com/pilsu-choi/mcpgraph/tree/archive/v1-daemon
tags: [decision, v2, daemon]
timestamp: 2026-10-18T07:05:00Z
---

# Archive the v1 daemon, store and API

**Decision.** `internal/daemon`, `internal/store`, `internal/api`, `internal/collector/watch` and
`cmd/mcpgraphd` were removed from `dev`. The last commit that has them, `a33015b`, is the branch
[`archive/v1-daemon`](https://github.com/pilsu-choi/mcpgraph/tree/archive/v1-daemon).

**Why.** v2 makes the daemon a stretch goal (milestone 12), wants one binary, and has no package
for a store or an API. Three options were offered:

- (a) keep the code as an early milestone 12;
- (b) archive it on a branch and remove it from the main line;
- (c) delete it.

The owner chose (b). The main line matches the spec, and nothing is lost.

**Consequences.**

- `fsnotify` and `modernc.org/sqlite` left `go.mod`. Both stay approved for milestone 12.
- The leak test no longer checks a SQLite file. It now checks `graph.Diff` output, which drift
  alerts will print.
- `scripts/demo.sh` became `scripts/docs.sh` without the daemon/API scenario. The logs were
  renumbered 07–10.
- Hard rule 3 still says the local API listens on 127.0.0.1 only. The
  [loopback-only decision](loopback-only-api.md) applies again when the API comes back.
- Wiki pages for the archived packages stay, marked *archived*: [daemon](../components/daemon.md),
  [store](../components/store.md), [api](../components/api.md), [watcher](../components/watcher.md).
