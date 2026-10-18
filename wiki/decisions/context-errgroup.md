---
type: Design Decision
title: One root context and errgroup
description: A single cancellable context flows everywhere; errgroup ties goroutine lifetimes together.
tags: [decision, concurrency, go]
timestamp: 2026-10-18T06:10:36Z
---

# One root context and errgroup

## Decision
`signal.NotifyContext` creates the root context in both binaries. All goroutines run under
`errgroup.WithContext`. Every blocking send or receive also `select`s on `ctx.Done()`.

## Why
- Ctrl-C stops everything promptly and cleanly (the daemon exits 0 on SIGINT; see `docs/logs/07-daemon.log`).
- When one component fails, the others stop, instead of leaking goroutines.

## But partial failure is not an error
A broken config file or an unreadable process table becomes an `Event.Warnings` entry. AGENTS.md
requires this: "a broken config for one app must not fail the whole scan". Tested by
`TestBrokenConfigBecomesWarning` and `TestListErrorIsWarning`.
