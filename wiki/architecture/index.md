---
type: Index
title: Architecture
description: How data flows from collectors through the single-writer builder to rules and outputs.
tags: [architecture]
timestamp: 2026-10-18T06:50:00Z
---

# Architecture

- [pipeline.md](pipeline.md): collectors → channel → builder → consumers, for one-shot scans and
  for the daemon.
- [graph-model.md](graph-model.md): node kinds, edge kinds, stable IDs.
- [concurrency.md](concurrency.md): goroutines, channel ownership, cancellation.

The reasons behind the shape are in [decisions/](../decisions/index.md).
