---
type: Go Package
title: internal/api
description: Read-only JSON API on 127.0.0.1: /graph, /findings, /snapshots.
resource: https://github.com/pilsu-choi/mcpgraph/blob/archive/v1-daemon/internal/api
tags: [component, go, archived]
timestamp: 2026-10-18T07:05:00Z
---

# `internal/api`

> **Archived.** This package lives on the `archive/v1-daemon` branch, not on `dev` ([decision](../decisions/archive-v1-daemon.md)). It returns with stretch milestone 12.

Read-only JSON API on 127.0.0.1: /graph, /findings, /snapshots.

Defines its own small interfaces (`StateSource`, `SnapshotLister`) instead of importing the daemon. See [loopback-only API](../decisions/loopback-only-api.md).
