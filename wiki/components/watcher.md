---
type: Go Package
title: internal/collector/watch
description: fsnotify watcher that re-parses changed config files after a debounce.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/collector/watch
tags: [component, go]
timestamp: 2026-10-18T06:50:00Z
---

# `internal/collector/watch`

fsnotify watcher that re-parses changed config files after a debounce.

Watches parent directories, because editors replace files by rename. One timer is shared by all files; each event pushes the deadline back (300 ms by default). On fire, it re-scans every pending file and sends one Event each. Directories that do not exist yet are left to the daemon's periodic re-scan.
