---
type: Go Package
title: internal/posture
description: Rule interface, Default set, Evaluate and HasHigh.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/posture
tags: [component, go]
timestamp: 2026-10-18T07:05:00Z
---

# `internal/posture`

> Renamed from `internal/rules` for v2 ([decision](../decisions/v2-transition.md)). `Default` now returns R1–R7.

Rule interface, Default set, Evaluate and HasHigh.

One file per rule, each with a table-driven test. `Evaluate` sorts by severity, then rule ID. R5 takes injectable `Exists`/`Glob` funcs, but its tests use a real `t.TempDir()` home. See [rules](../rules/index.md).
