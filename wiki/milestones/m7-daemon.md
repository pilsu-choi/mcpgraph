---
type: Milestone
title: m7: Daemon
description: fsnotify watcher with debounce, periodic re-scan, SQLite store (pure Go), diff logging, `Sink` no-op, and the `mcpgraphd` binary.
resource: https://github.com/pilsu-choi/mcpgraph/commit/4635de7
tags: [milestone, m7]
timestamp: 2026-10-18T06:21:46Z
---

# m7: Daemon

**Commit:** [`4635de7`](https://github.com/pilsu-choi/mcpgraph/commit/4635de7)

## Delivered
fsnotify watcher with debounce, periodic re-scan, SQLite store (pure Go), diff logging, `Sink` no-op, and the `mcpgraphd` binary.

## Acceptance check
`TestConfigEditLogsDiffWithinTwoSeconds`: the diff is logged about 100 ms after the edit. The real binary exits 0 on SIGINT.

## Go concepts used
- `errgroup` with several long-running goroutines
- `atomic.Pointer[T]` for lock-free publishing
- Go 1.23 timer `Reset` semantics; `clear(map)`
- `database/sql` transactions with deferred Rollback
- Blank import for driver registration
