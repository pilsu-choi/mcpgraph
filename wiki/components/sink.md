---
type: Go Package
title: internal/sink
description: SUPERSEDED. The old no-op snapshot Sink interface; replaced by the opt-in audit-log sink in milestone 11.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/sink
tags: [component, go]
timestamp: 2026-10-18T06:45:00Z
---

# `internal/sink`

> **Superseded (2026-10-18).** The no-op `Sink` interface was removed in milestone 11. See
> [sink and collector](2026-10-18-sink-collector.md).

Interface for forwarding snapshots to a central server; only a no-op exists.

`var _ Sink = Nop{}` is a compile-time check that Nop satisfies the interface. A real Sink would break hard rule 3, so it is future work.
