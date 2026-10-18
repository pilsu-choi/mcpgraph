---
type: Index
title: Design decisions
description: Why mcpgraph is built the way it is.
tags: [decision]
timestamp: 2026-10-18T07:25:00Z
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
| [Move to AGENTS.md v2](v2-transition.md) | mcpgraph adds runtime control (proxy, hook, policy, audit) to posture. |
| [Archive the v1 daemon](archive-v1-daemon.md) | Daemon, store, API and watcher live on `archive/v1-daemon` until milestone 12. |
| [Action carries the working directory](action-cwd.md) | `policy.Action.Cwd` exists so recursive deletes can be judged against the project. |
| [Proxy denials are `isError` results](proxy-denial-iserror.md) | The model reliably sees why a tool call was blocked. |
| [Deny before ask](deny-before-ask.md) | Stricter rules first in `default.yaml`; `rm -rf ~/.ssh` is denied. |
| [Plain HTTP becomes R7](plain-http-as-r7.md) | R4 now means "not proxied"; the old check keeps its logic as R7. |

These match AGENTS.md §5 and the README's *Design decisions* section.
