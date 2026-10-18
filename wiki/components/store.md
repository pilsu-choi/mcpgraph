---
type: Go Package
title: internal/store
description: SQLite snapshots with changes, pruning to the last 500.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/store
tags: [component, go]
timestamp: 2026-10-18T06:50:00Z
---

# `internal/store`

SQLite snapshots with changes, pruning to the last 500.

Tables `snapshots` and `changes` (`ON DELETE CASCADE`). WAL mode, busy_timeout and foreign_keys are set as DSN pragmas. `Save` runs in one transaction (deferred Rollback is a no-op after Commit). Methods: `Latest` (for diffing after a restart) and `List` (for the API).
