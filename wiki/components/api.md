---
type: Go Package
title: internal/api
description: Read-only JSON API on 127.0.0.1: /graph, /findings, /snapshots.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/api
tags: [component, go]
timestamp: 2026-10-18T06:45:00Z
---

# `internal/api`

Read-only JSON API on 127.0.0.1: /graph, /findings, /snapshots.

Defines its own small interfaces (`StateSource`, `SnapshotLister`) instead of importing the daemon. See [loopback-only API](../decisions/loopback-only-api.md).
