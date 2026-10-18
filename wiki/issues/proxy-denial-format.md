---
type: Issue
title: How the proxy should answer a denied tools/call
description: AGENTS.md asks for a JSON-RPC error carrying the rule ID; the MCP spec says clients only MAY show JSON-RPC errors to the model but SHOULD show results with isError. Needs an owner decision before milestone 7.
tags: [issue, open, proxy, m7, decision-needed]
timestamp: 2026-10-18T07:05:00Z
---

# How the proxy should answer a denied `tools/call`

**Status:** open. The owner must decide before [milestone 7](../milestones/index.md).

The m7 acceptance check in AGENTS.md says a denied `tools/call` is "answered with a JSON-RPC error
that carries the rule ID". The [format research](../research/external-formats.md) (§A4) found:

- The MCP spec says tool *execution* errors go in a normal result with `isError: true`, and clients
  **SHOULD** pass those to the model.
- Clients only **MAY** pass JSON-RPC (protocol) errors to the model. So with a JSON-RPC error, the
  agent may never see *why* it was blocked, and the demo needs it to see that.
- If a JSON-RPC error is used, the code must be outside `-32768..-32000`, or the generic `-32603`.
  The range `-32000..-32099` is legacy or reserved.

**Options:**

1. JSON-RPC error, as written in the spec, with a custom code (e.g. `-31001`) and
   `data: {rule_id, verdict}`.
2. Result with `isError: true` and a text item such as `blocked by mcpgraph rule
   block-destructive-sql: ...`, plus `_meta` carrying the rule ID. The model reliably sees it.
3. Make it configurable, with (2) as the default.

**Recommendation:** (2). The rule ID still appears in the message and in `_meta`, so the
acceptance test can check it.
