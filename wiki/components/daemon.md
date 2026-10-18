---
type: Go Package
title: internal/daemon
description: Long-running pipeline: initial full scan, watcher, periodic re-scan, publish.
resource: https://github.com/pilsu-choi/mcpgraph/blob/archive/v1-daemon/internal/daemon
tags: [component, go, archived]
timestamp: 2026-10-18T07:05:00Z
---

# `internal/daemon`

> **Archived.** This package lives on the `archive/v1-daemon` branch, not on `dev` ([decision](../decisions/archive-v1-daemon.md)). It returns with stretch milestone 12.

Long-running pipeline: initial full scan, watcher, periodic re-scan, publish.

`publish` evaluates rules and stores `State` in an `atomic.Pointer`. It diffs against the last saved graph, logs changes (process churn at debug level), saves a snapshot only when something changed, and forwards to the [Sink](sink.md). Acceptance test: `TestConfigEditLogsDiffWithinTwoSeconds`.
