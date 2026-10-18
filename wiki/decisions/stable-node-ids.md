---
type: Design Decision
title: Stable, content-derived node IDs
description: ID = kind + ':' + sha256(stable key)[:12] so the same thing has the same ID across scans.
tags: [decision, model, diff]
timestamp: 2026-10-18T06:04:58Z
---

# Stable, content-derived node IDs

## Decision
`model.NodeID(kind, key)` hashes a stable key:

- servers: config path + JSON path,
- processes: pid + create time,
- credentials: location,
- data sources: kind + identifier.

## Why
- `graph.Diff` becomes a comparison by ID across two snapshots ("added", "removed", "changed").
- Two servers that share a data source dedupe to one node.
- Using pid + create time means a reused pid is a different process.

## Consequence
Fixture IDs change when the fake home path changes (the path is part of the key). That is why the
README Mermaid sample has different hashes than a fresh run.
