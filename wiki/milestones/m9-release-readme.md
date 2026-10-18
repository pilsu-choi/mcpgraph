---
type: Milestone
title: m9: Release + README
description: `.
resource: https://github.com/pilsu-choi/mcpgraph/commit/0a474ab
tags: [milestone, m9]
timestamp: 2026-10-18T06:24:48Z
---

# m9: Release + README

**Commit:** [`0a474ab`](https://github.com/pilsu-choi/mcpgraph/commit/0a474ab)

## Delivered
`.goreleaser.yaml` (v2, YAML anchors for two builds), README with the problem, architecture, decisions, sample output, limitations and future work.

## Acceptance check
`goreleaser build --snapshot --clean` builds 8 static, stripped binaries (2 commands × darwin/linux × amd64/arm64).

## Go concepts used
- `CGO_ENABLED=0` cross-compilation
- `-trimpath`, `-s -w` ldflags
