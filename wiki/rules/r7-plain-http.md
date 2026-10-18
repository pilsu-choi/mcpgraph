---
type: Risk Rule
title: R7: Remote MCP over plain http://
description: A server URL with scheme `http` and a host that is not loopback (localhost, 127/8, ::1).
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/posture/r7_plain_http.go
tags: [rule, r7, medium]
timestamp: 2026-10-18T07:05:00Z
---

# R7: Remote MCP over plain http://

- **History:** This was R4 in v1. It was renumbered when v2 redefined R4 ([decision](../decisions/plain-http-as-r7.md)).
- **Severity:** medium
- **Detects:** A server URL with scheme `http` and a host that is not loopback (localhost, 127/8, ::1).
- **Rationale (shown in the finding):** Tool calls, results and auth headers can be read or changed in transit.
- **Test cases:** http to IP or domain → 1; https, localhost, 127.0.0.1, [::1], empty → 0.

Source: [`internal/posture/r7_plain_http.go`](https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/posture/r7_plain_http.go). Rule interface and ordering: [rules component](../components/posture.md).
