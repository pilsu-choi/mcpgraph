---
type: Index
title: Issues
description: Open questions and known gaps.
tags: [issue]
timestamp: 2026-10-18T07:05:00Z
---

# Issues

| Issue | Status |
| --- | --- |
| [Most config paths are unverified](config-path-verification.md) | open |
| [Race detector cannot run locally](race-detector-local.md) | open |
| [Plugin-provided MCP servers are not in configs](plugin-servers-invisible.md) | open |

Also: the README *Limitations* section.
| [Proxy denial format](proxy-denial-format.md) | JSON-RPC error vs `isError` result for a denied `tools/call`; owner decision needed before m7. |
| [MCP revision per client](mcp-revision-per-client.md) | 2026-07-28 vs 2025-11-25; the proxy must work with both. |
| [Hooks fail open](hook-fails-open.md) | Claude Code lets the call through when a hook fails, unless `onFailure: block`. |
