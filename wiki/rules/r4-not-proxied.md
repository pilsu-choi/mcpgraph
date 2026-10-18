---
type: Risk Rule
title: R4: MCP server not routed through the proxy
description: A stdio MCP server whose config runs it directly instead of through `mcpgraph proxy --`.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/posture/r4_not_proxied.go
tags: [rule, r4, medium, v2]
timestamp: 2026-10-18T07:05:00Z
---

# R4: MCP server not routed through the proxy

- **Severity:** medium
- **Detects:** A stdio MCP server with `MCPServer.Proxied == false`. Remote (http/sse) servers are
  skipped, because the proxy handles stdio only and nobody could fix that finding.
- **How "proxied" is decided:** the config collector recognises entries that `mcpgraph install`
  will write: `command` is `mcpgraph` (any directory) and `args` is `["proxy", <flags...>, "--",
  <server command...>]`. It then records the *wrapped* command on the node, so R1, R3 and R5 still
  see what really runs (`internal/collector/config/proxied.go`). mcpgraph defines this format
  itself, so it needs no external verification.
- **Rationale (shown in the finding):** Its tool calls reach the server without a policy check or
  an audit record. Run `mcpgraph install` to wrap it.
- **Test cases:** direct stdio → 1; proxied stdio, http, sse → 0. The unwrapping has its own table
  test, covering a missing `--`, nothing after `--`, and other binaries.
- **Until milestone 8** every stdio server triggers R4: 6 of the 10 medium findings on the fixtures.

Replaces the v1 R4 ([decision](../decisions/plain-http-as-r7.md)). Source:
[`internal/posture/r4_not_proxied.go`](https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/posture/r4_not_proxied.go).
