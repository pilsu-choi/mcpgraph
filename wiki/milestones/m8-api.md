---
type: Milestone
title: m8: Local API
description: `GET /graph`, `/findings`, `/snapshots` on 127.
resource: https://github.com/pilsu-choi/mcpgraph/commit/92ece4b
tags: [milestone, m8]
timestamp: 2026-10-18T06:23:01Z
---

# m8: Local API

**Commit:** [`92ece4b`](https://github.com/pilsu-choi/mcpgraph/commit/92ece4b)

## Delivered
`GET /graph`, `/findings`, `/snapshots` on 127.0.0.1 only, Host-header check, graceful shutdown.

## Acceptance check
`TestRefusesNonLoopbackAddress` rejects 0.0.0.0, `:port`, [::], LAN IPs, localhost and bad ports. With curl: POST → 405, foreign Host → 403.

## Go concepts used
- Go 1.22 method routing patterns
- Middleware as `http.Handler` wrapper
- `httptest.NewRecorder`
- Compile-time interface checks `var _ I = T{}`
