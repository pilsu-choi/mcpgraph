---
type: Go Package
title: internal/store
description: SQLite snapshots with changes, pruning to the last 500.
resource: https://github.com/pilsu-choi/mcpgraph/blob/archive/v1-daemon/internal/store
tags: [component, go, archived]
timestamp: 2026-10-18T07:05:00Z
---

# `internal/store`

> **Archived.** This package lives on the `archive/v1-daemon` branch, not on `dev` ([decision](../decisions/archive-v1-daemon.md)). It returns with stretch milestone 12.

SQLite snapshots with changes, pruning to the last 500.

Tables `snapshots` and `changes` (`ON DELETE CASCADE`). WAL mode, busy_timeout and foreign_keys are set as DSN pragmas. `Save` runs in one transaction (deferred Rollback is a no-op after Commit). Methods: `Latest` (for diffing after a restart) and `List` (for the API).
