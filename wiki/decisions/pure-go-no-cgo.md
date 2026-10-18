---
type: Design Decision
title: Pure Go, no cgo
description: gopsutil and modernc.org/sqlite keep CGO_ENABLED=0 static cross-builds possible.
tags: [decision, build, release]
timestamp: 2026-10-18T06:24:48Z
---

# Pure Go, no cgo

## Decision
All dependencies are pure Go. `modernc.org/sqlite` is a transpiled SQLite, and gopsutil reads
`/proc` and sysctl. All builds use `CGO_ENABLED=0`.

## Why
- One static binary for each of darwin/linux × amd64/arm64, built from a single Linux host with GoReleaser.
- Endpoint agents must not depend on the host's libc version.

## Cost
The race detector needs cgo **at test time only**. CI runs `go test -race`. Locally it is skipped
because there is no C compiler ([issue](../issues/race-detector-local.md)).
