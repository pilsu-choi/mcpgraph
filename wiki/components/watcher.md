---
type: Go Package
title: internal/collector/watch
description: fsnotify watcher that re-parses changed config files after a debounce.
resource: https://github.com/pilsu-choi/mcpgraph/blob/archive/v1-daemon/internal/collector/watch
tags: [component, go, archived]
timestamp: 2026-10-18T07:05:00Z
---

# `internal/collector/watch`

> **Archived.** This package lives on the `archive/v1-daemon` branch, not on `dev` ([decision](../decisions/archive-v1-daemon.md)). It returns with stretch milestone 12.

fsnotify watcher that re-parses changed config files after a debounce.

Watches parent directories, because editors replace files by rename. One timer is shared by all files; each event pushes the deadline back (300 ms by default). On fire, it re-scans every pending file and sends one Event each. Directories that do not exist yet are left to the daemon's periodic re-scan.
