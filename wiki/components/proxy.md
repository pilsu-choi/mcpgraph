---
type: Go Package
title: internal/proxy
description: stdio MCP proxy that judges tools/call with the policy engine and forwards every other message byte for byte.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/proxy
tags: [component, go, proxy, v2, m7]
timestamp: 2026-10-18T07:35:00Z
---

# `internal/proxy`

| File | Role |
| --- | --- |
| `framing.go` | `lineReader` returns each raw line, newline included, with no size limit; `syncWriter` serialises writes to the client. |
| `message.go` | Exact-key parsing (`map[string]json.RawMessage`), `idKey`, `parseToolCall`, `denialResponse`. |
| `proxy.go` | `Config`, `Proxy.Run`, the two copy loops, judging, batches, tool inventory, audit. |
| `exec.go` | `RunCommand`: start the server, wire its pipes, propagate its exit code. |

## Flow

- **client → server.** Read a line and parse it. If it is a `tools/call` request, build a
  `policy.Action` (`Source: proxy`, `Agent: --server`, `App: --app`) and call `policy.Judge`.
  Allow → forward the original bytes. Deny or ask → write an `isError` result to the client, remember
  the ID in `answered`, and never forward. A `notifications/cancelled` for an answered ID is
  dropped. Anything else, including lines that are not JSON, is forwarded unchanged.
- **server → client.** Forward every line unchanged. Responses to `tools/list` are also read to
  learn tool names (union across pages).
- `Run` returns when the server side ends; the client side's `defer` closes the server's stdin.

## Choices

- Denials use `resultType: "complete"` only when the request carried the 2026-07-28 version in
  `_meta`.
- `KnownTool` returns true until the first `tools/list` answer arrives.
- Every decision is recorded through the `Recorder` interface (`audit.Log` in the CLI).

Related: [policy](policy.md), [audit](audit.md), [milestone](../milestones/v2-m7-proxy.md).
