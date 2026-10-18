---
type: Go Package
title: internal/model
description: Shared types: Node, Edge, Graph, Event, Finding, NodeID.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/model
tags: [component, go]
timestamp: 2026-10-18T07:05:00Z
---

> **v2 additions:** `KindDecision`, `EdgeDecidedOn` (Decision → MCPServer or AIApp), `MCPServer.Proxied`, and the `Decision` type with `Verdict` (`allow`/`deny`/`ask`) and source (`proxy`/`hook`). The summary on a decision is redacted before the node is created.

# `internal/model`

Shared types: Node, Edge, Graph, Event, Finding, NodeID.

No dependencies on other mcpgraph packages, so anything can import it. `Graph.Snapshot()` deep-copies pointer fields and slices. Lookup helpers (`Node`, `NodesOfKind`, `Out`, `In`) are plain loops. See [graph model](../architecture/graph-model.md).
