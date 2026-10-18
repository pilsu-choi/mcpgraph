---
type: Architecture
title: Concurrency model
description: Which goroutine owns what, who closes channels, and how cancellation spreads.
tags: [architecture, concurrency, go]
timestamp: 2026-10-18T06:50:00Z
---

# Concurrency model

| Thing | Owner | Others may |
| --- | --- | --- |
| Builder state (`sources` map) | builder goroutine | nothing; they send Events |
| Published `*model.Graph` | nobody (immutable) | read freely; `Snapshot()` for a private copy |
| `chan model.Event` | its creator (scan orchestrator / daemon) | send only; never close |
| Daemon current state | builder goroutine writes via `atomic.Pointer[State]` | API handlers `Load()` |
| `*sql.DB` | shared | it is a pool with its own locking |

Rules followed:

- **Only the creator closes a channel.** Collectors share the channel, so none of them may close it.
- **Every send `select`s on `ctx.Done()`**, so a send never blocks after the reader stops.
- **`errgroup.WithContext`**: the first error cancels the shared context, and `Wait` returns that error.
- **Happens-before via close.** In `scan.Collect`, the error variable is written before
  `close(events)` and read after the range loop ends, so no lock is needed.

Proof: `internal/graph/builder_test.go` sends from 20 goroutines and is meant to run with `-race`
(in CI; see [issue](../issues/race-detector-local.md)).
