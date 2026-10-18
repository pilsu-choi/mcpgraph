---
type: Issue
title: Which MCP revision each client speaks
description: MCP 2026-07-28 removed the initialize handshake and server-to-client requests; real clients may speak either revision. The proxy must be era-agnostic; real traffic should be captured.
tags: [issue, open, proxy, verify]
timestamp: 2026-10-18T07:35:00Z
---

# Which MCP revision each client speaks

**Status:** open, partly answered. The proxy works with both revisions (milestone 7). The official
TypeScript SDK 1.30.0 client negotiates `2025-11-25`; its capture is in
`testdata/jsonrpc/captured-sdk-1.30.0.jsonl`. The AI apps themselves are still unchecked.

The latest MCP revision, `2026-07-28`, is stateless:

- There is no `initialize` handshake. Instead, `server/discover` and per-request `_meta` carry the
  protocol version and capabilities.
- The server sends no requests to the client. Sampling, elicitation and roots become
  `input_required` results.
- `ping` and `logging/setLevel` were removed.

The previous revision, `2025-11-25`, does all of these the old way
([research](../research/external-formats.md) §A5 and §A8).

Official docs do not say which revision Claude Desktop, Cursor, Claude Code and VS Code speak today.

**Plan.** Make the proxy independent of the revision:

- Forward everything except `tools/call` byte for byte.
- Track in-flight IDs separately for each direction, because in the legacy revision servers send
  requests too.
- Keep IDs as `json.RawMessage`.
- Read lines with a large buffer, because the spec sets no maximum size.

Then record a real transcript for each client into `testdata/` and mark it verified.

## Update (m7)

- Done: everything except `tools/call` is forwarded byte for byte; IDs are kept raw; lines have no
  size limit. Only client requests need tracking: a server request is forwarded as is, and the
  client's answer to it is not a `tools/call`.
- Recorded: SDK 1.30.0 client ↔ server (verified, real traffic).
- Still to do: capture Claude Desktop, Cursor, Claude Code and VS Code through
  `mcpgraph proxy` once `install` (milestone 8) can wrap them.
