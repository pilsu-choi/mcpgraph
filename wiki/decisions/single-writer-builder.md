---
type: Design Decision
title: Single-writer graph builder
description: Only the builder goroutine mutates graph state; collectors send Events; readers get immutable graphs.
tags: [decision, concurrency]
timestamp: 2026-10-18T06:16:12Z
---

# Single-writer graph builder

## Decision
One goroutine owns the graph. Collectors send `model.Event` over a channel. Each publish builds a
fresh `*model.Graph` that shares no memory with builder state.

## Why
- No mutex on the graph, so no lock ordering, no forgotten unlock, and nothing for readers to block on.
- Per-source replacement makes removals easy: a re-parsed config replaces its old contribution.
- Immutable published graphs can go to rules, the API and the store without copying under a lock.

## Trade-offs
- A full rebuild on every publish costs O(nodes + edges). Graphs here have tens to hundreds of
  nodes, so this is cheap.
- Lookups use linear scans instead of an index (see [graph model](../architecture/graph-model.md)).

Implemented in [graph builder](../components/graph-builder.md). Tested by `TestRunConcurrentProducers`.
