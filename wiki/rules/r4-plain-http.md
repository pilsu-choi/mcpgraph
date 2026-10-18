---
type: Risk Rule
title: R4: Remote MCP over plain http://
description: A server URL with scheme `http` and a host that is not loopback (localhost, 127/8, ::1).
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/rules/r4_plain_http.go
tags: [rule, r4, medium]
timestamp: 2026-10-18T06:18:47Z
---

# R4: Remote MCP over plain http://

- **Severity:** medium
- **Detects:** A server URL with scheme `http` and a host that is not loopback (localhost, 127/8, ::1).
- **Rationale (shown in the finding):** Tool calls, results and auth headers can be read or changed in transit.
- **Test cases:** http to IP or domain → 1; https, localhost, 127.0.0.1, [::1], empty → 0.

Source: [`internal/rules/r4_plain_http.go`](https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/rules/r4_plain_http.go). Rule interface and ordering: [rules component](../components/rules.md).
