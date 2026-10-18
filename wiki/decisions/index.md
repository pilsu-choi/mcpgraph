---
type: Index
title: Design decisions
description: Why mcpgraph is built the way it is.
tags: [decision]
timestamp: 2026-10-18T06:50:00Z
---

# Design decisions

| Decision | Summary |
| --- | --- |
| [Single-writer graph builder](single-writer-builder.md) | Only the builder goroutine mutates graph state; collectors send Events; readers get immutable graphs. |
| [One root context and errgroup](context-errgroup.md) | A single cancellable context flows everywhere; errgroup ties goroutine lifetimes together. |
| [Collectors and parsers are small interfaces](collector-interfaces.md) | Collector{Name, Collect} and parser{App, Parse} keep adding a source or app local to one file. |
| [Stable, content-derived node IDs](stable-node-ids.md) | ID = kind + ':' + sha256(stable key)[:12] so the same thing has the same ID across scans. |
| [Redact secrets at collection time](redact-at-collection.md) | Raw config values never leave the config package; every stored string is masked first. |
| [Loopback-only, read-only API](loopback-only-api.md) | The API binds only 127.0.0.1, checks the Host header, and exposes GET routes only. |
| [Pure Go, no cgo](pure-go-no-cgo.md) | gopsutil and modernc.org/sqlite keep CGO_ENABLED=0 static cross-builds possible. |

These match AGENTS.md §5 and the README's *Design decisions* section.
