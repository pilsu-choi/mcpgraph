---
type: Milestone
title: m5: Graph builder + exports
description: Builder tests (replacement, dedupe, determinism, concurrency, cancellation), `graph.
resource: https://github.com/pilsu-choi/mcpgraph/commit/8bae004
tags: [milestone, m5]
timestamp: 2026-10-18T06:16:12Z
---

# m5: Graph builder + exports

**Commit:** [`8bae004`](https://github.com/pilsu-choi/mcpgraph/commit/8bae004)

## Delivered
Builder tests (replacement, dedupe, determinism, concurrency, cancellation), `graph.Diff`, JSON and Mermaid export, and the `export` command.

## Acceptance check
The Mermaid export (80 lines including processes) parses with mermaid 11. No browser was installed at that point; later rendered in headless Chromium ([m10](m10-docs-visuals.md)).

## Go concepts used
- Single-writer goroutine
- Non-blocking receive with `select`/`default`
- Comparable struct as a map key (`map[model.Edge]bool`)
- `sync.WaitGroup` in tests; `-race`
