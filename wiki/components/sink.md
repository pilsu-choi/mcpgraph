---
type: Go Package
title: internal/sink
description: Interface for forwarding snapshots to a central server; only a no-op exists.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/sink
tags: [component, go]
timestamp: 2026-10-18T06:45:00Z
---

# `internal/sink`

Interface for forwarding snapshots to a central server; only a no-op exists.

`var _ Sink = Nop{}` is a compile-time check that Nop satisfies the interface. A real Sink would break hard rule 3, so it is future work.
