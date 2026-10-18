---
type: Decision
title: Keep the plain-HTTP rule as R7
description: v2 reuses the ID R4 for "not routed through the proxy"; the v1 plain-http check keeps its logic under the new ID R7.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/posture/r7_plain_http.go
tags: [decision, v2, rule, r4, r7]
timestamp: 2026-10-18T07:05:00Z
---

# Keep the plain-HTTP rule as R7

**Problem.** In v1, R4 flagged a remote MCP server reached over unencrypted `http://`. v2 gives the
ID R4 to a different check: "MCP server not routed through the proxy". The two checks have nothing
in common except the number.

**Options.** Drop the plain-HTTP check, so the rules match v2's table exactly, or keep it under a new
ID.

**Decision.** The owner kept it as [R7](../rules/r7-plain-http.md). The code and tests already
existed, so keeping it costs almost nothing, and it is still a useful posture check. v2 puts remote
HTTP servers out of scope for the *proxy*, but not for posture. The new
[R4](../rules/r4-not-proxied.md) is a separate file.
