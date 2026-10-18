---
type: Index
title: Issues
description: Open questions and known gaps.
tags: [issue]
timestamp: 2026-10-18T08:30:00Z
---

# Issues

| Issue | Status |
| --- | --- |
| [Most config paths are unverified](config-path-verification.md) | open |
| [Race detector cannot run locally](race-detector-local.md) | open |
| [Plugin-provided MCP servers are not in configs](plugin-servers-invisible.md) | open |
| [Proxy denial format](proxy-denial-format.md) | *Resolved:* `isError` result ([decision](../decisions/proxy-denial-iserror.md)). |
| [MCP revision per client](mcp-revision-per-client.md) | 2026-07-28 vs 2025-11-25; the proxy works with both. SDK 1.30.0 speaks 2025-11-25; the AI apps are unchecked. |
| [Hooks fail open](hook-fails-open.md) | *Mitigated:* `--print-config` emits `onFailure: block` and the hook exits 2 on any internal error. Risk remains for hand-written configs. |
| [Codex hook unverified](2026-10-18-codex-hook-unverified.md) | open: a real `codex exec` did not invoke the project-local hook. |

Also: the README *Limitations* section.
