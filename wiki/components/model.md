---
type: Go Package
title: internal/model
description: Shared types: Node, Edge, Graph, Event, Finding, NodeID.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/model
tags: [component, go]
timestamp: 2026-10-18T06:45:00Z
---

# `internal/model`

Shared types: Node, Edge, Graph, Event, Finding, NodeID.

No dependencies on other mcpgraph packages, so anything can import it. `Graph.Snapshot()` deep-copies pointer fields and slices. Lookup helpers (`Node`, `NodesOfKind`, `Out`, `In`) are plain loops. See [graph model](../architecture/graph-model.md).
