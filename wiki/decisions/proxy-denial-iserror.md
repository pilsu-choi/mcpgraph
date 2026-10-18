---
type: Decision
title: Proxy denials are isError results, not JSON-RPC errors
description: A denied or ask-ed tools/call is answered with a normal result marked isError: true, so the model reliably sees the reason; this supersedes the JSON-RPC error wording of milestone 7.
tags: [decision, proxy, m7, owner]
timestamp: 2026-10-18T07:25:00Z
---

# Proxy denials are `isError` results, not JSON-RPC errors

**Decided by the owner on 2026-10-18.** Recorded in AGENTS.md §15, which overrides the milestone 7
acceptance wording.

When the proxy denies a `tools/call`, or asks for approval, it answers in place of the server:

```json
{"jsonrpc":"2.0","id":<same id>,"result":{
  "content":[{"type":"text","text":"Blocked by mcpgraph rule block-destructive-sql: ..."}],
  "isError":true,
  "_meta":{"mcpgraph/rule_id":"block-destructive-sql","mcpgraph/verdict":"deny"}}}
```

The exact `_meta` key names are fixed in m7. For revision 2026-07-28 the result also carries
`"resultType":"complete"`.

**Why.** The [format research](../research/external-formats.md) (§A4) found that clients
**SHOULD** pass tool-execution errors (`isError`) to the model, but only **MAY** pass JSON-RPC
errors. The demo requires that "the agent is told why", so the error must be one the model sees.

**Consequences.**

- m7's acceptance test checks for an `isError` result whose text and `_meta` carry the rule ID.
- No custom JSON-RPC error code is needed, which avoids the reserved ranges.
- Closes [issue: proxy denial format](../issues/proxy-denial-format.md).
