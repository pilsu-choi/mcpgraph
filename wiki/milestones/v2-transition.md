---
type: Milestone
title: v2 transition: spec, layout and skeleton
description: Adopted AGENTS.md v2: archived the daemon, renamed rules to posture, added R4 (not proxied) and kept plain http as R7, added Decision types, CLI stubs, make bench/docs.
tags: [milestone, v2, m1, m4]
timestamp: 2026-10-18T07:05:00Z
---

# v2 transition: spec, layout and skeleton

Brings the v1 code in line with [AGENTS.md v2](../decisions/v2-transition.md) before new
milestones start.

## How v1 work maps onto the v2 milestones

| v2 milestone | v1 work that covers it | Still missing after this change |
| --- | --- | --- |
| 1 Skeleton | v1 m1 | — (stubs, `make bench` and model types added here) |
| 2 Discovery | v1 m2 + m4 | Config path verification beyond `~/.claude.json` ([issue](../issues/config-path-verification.md)) |
| 3 Redaction | v1 m3 | Tool-call args and shell commands (milestone 6 uses `redact`) |
| 4 Graph + posture | v1 m5 + m6 | — (R4 redefined, R7 added here) |
| 5–10 | — | everything |
| 12 Daemon (stretch) | v1 m7 + m8 | archived ([decision](../decisions/archive-v1-daemon.md)) |

## Changes

- **Spec:** AGENTS.md replaced with v2. It keeps §14 (wiki) and adds §15 (owner decisions).
- **Archive:** branch `archive/v1-daemon` at `a33015b`. `internal/{daemon,store,api}`,
  `internal/collector/watch` and `cmd/mcpgraphd` were removed, and `go mod tidy` dropped fsnotify
  and sqlite.
- **Model:** added `KindDecision`, `EdgeDecidedOn`, `MCPServer.Proxied`, the `Decision` struct,
  `Verdict` and source constants. `Snapshot()` deep-copies decisions; Mermaid styles Decision nodes.
- **Posture:** `internal/rules` → `internal/posture`. [R4](../rules/r4-not-proxied.md) is new, and
  plain HTTP moved to [R7](../rules/r7-plain-http.md). `unwrapProxy` in the config collector
  recognises `mcpgraph proxy -- <cmd>` entries.
- **CLI:** `proxy`, `hook`, `install`, `uninstall` and `events` are stubs that return
  `not implemented yet (milestone mN)`.
- **Make:** added `bench` (policy benchmarks) and `docs` (old `demo`, now `scripts/docs.sh`). `demo`
  is reserved for milestone 10 and fails with a pointer for now.
- **Docs:** logs renumbered 07–10. README updated: two-job intro, R4/R7, Decision in the graph
  model, roadmap, no daemon/API section.
- **Research:** [external formats](../research/external-formats.md) checked by a research
  subagent. Three issues opened: [proxy denial format](../issues/proxy-denial-format.md),
  [MCP revision per client](../issues/mcp-revision-per-client.md),
  [hooks fail open](../issues/hook-fails-open.md).

## Check

`go build`, `golangci-lint run` (0 issues), `go test ./...` and `scripts/docs.sh` (leak check clean).
The fixture report now shows 9 high and 10 medium findings; R4 accounts for 6 of the medium ones.

## Go concepts

- **Named string types as enums** (`Verdict`, `NodeKind`): the compiler rejects mixing them up.
- **Copying a slice to avoid aliasing** (`append([]string(nil), s.Args[i+2:]...)`): re-slicing
  would share the backing array with the caller.
- **Closures that return functions** (`notYet("m7")` returns a cobra `RunE`).
- **Package rename = directory rename + `package` clause + import paths.** Go has no separate
  module manifest per package.
