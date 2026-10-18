---
type: Go Package
title: internal/daemon
description: Long-running pipeline: initial full scan, watcher, periodic re-scan, publish.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/daemon
tags: [component, go]
timestamp: 2026-10-18T06:50:00Z
---

# `internal/daemon`

Long-running pipeline: initial full scan, watcher, periodic re-scan, publish.

`publish` evaluates rules and stores `State` in an `atomic.Pointer`. It diffs against the last saved graph, logs changes (process churn at debug level), saves a snapshot only when something changed, and forwards to the [Sink](sink.md). Acceptance test: `TestConfigEditLogsDiffWithinTwoSeconds`.
