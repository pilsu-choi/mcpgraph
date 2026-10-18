---
type: Go Package
title: internal/rules
description: Rule interface, Default set, Evaluate and HasHigh.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/rules
tags: [component, go]
timestamp: 2026-10-18T06:50:00Z
---

# `internal/rules`

Rule interface, Default set, Evaluate and HasHigh.

One file per rule, each with a table-driven test. `Evaluate` sorts by severity, then rule ID. R5 takes injectable `Exists`/`Glob` funcs, but its tests use a real `t.TempDir()` home. See [rules](../rules/index.md).
