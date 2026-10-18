---
type: Milestone
title: m1: Skeleton
description: go.
resource: https://github.com/pilsu-choi/mcpgraph/commit/d1c0170
tags: [milestone, m1]
timestamp: 2026-10-18T06:04:58Z
---

# m1: Skeleton

**Commit:** [`d1c0170`](https://github.com/pilsu-choi/mcpgraph/commit/d1c0170)

## Delivered
go.mod (Go 1.27.2), directory layout, `model` types with tests, a cobra CLI with a `scan` stub, Makefile, `.golangci.yml` (v2), GitHub Actions CI (lint, `-race` tests, cgo-free build).

## Acceptance check
Build, `golangci-lint run` and `go test ./...` pass. `make` itself is not installed, so the same commands were run directly.

## Go concepts used
- Named string types (`NodeKind`) for compile-time safety
- Struct with optional pointer fields as a tagged union
- Value vs pointer receivers; deep copy in `Snapshot()`
- `errors.As` to carry an exit code through cobra
