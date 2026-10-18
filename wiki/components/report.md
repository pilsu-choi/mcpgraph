---
type: Go Package
title: internal/report
description: Inventory and findings tables, JSON, Mermaid.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/report
tags: [component, go]
timestamp: 2026-10-18T06:50:00Z
---

# `internal/report`

Inventory and findings tables, JSON, Mermaid.

Tables use `text/tabwriter`. Mermaid escapes `#`, quotes and angle brackets as entity codes (`#35;`, `#quot;`, ...) in a single `strings.NewReplacer` pass, and styles each node kind with `classDef`.
