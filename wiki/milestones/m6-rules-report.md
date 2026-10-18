---
type: Milestone
title: m6: Rules R1–R6 + report
description: Six rules, one file each with table tests.
resource: https://github.com/pilsu-choi/mcpgraph/commit/6af1a60
tags: [milestone, m6]
timestamp: 2026-10-18T06:18:47Z
---

# m6: Rules R1–R6 + report

**Commit:** [`6af1a60`](https://github.com/pilsu-choi/mcpgraph/commit/6af1a60)

## Delivered
Six rules, one file each with table tests. Findings table grouped by severity, the `report` command (table/json), exit code 2 on high, CLI tests, and the shared `netutil.IsLoopbackHost`.

## Acceptance check
Every rule has positive and negative cases. The built binary exits 2 on the fixtures.

## Go concepts used
- Small interfaces (`Rule`)
- Function-typed fields for injectable dependencies (R5)
- `sort.SliceStable`
- `filepath.Rel` for safe containment checks
