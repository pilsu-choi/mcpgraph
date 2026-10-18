---
type: Issue
title: Most config paths are unverified
description: Only ~/.claude.json is confirmed; other candidates are TODO(verify).
tags: [issue, open]
timestamp: 2026-10-18T06:45:00Z
---

# Most config paths are unverified

| App | Path | Status |
| --- | --- | --- |
| Claude Code (Linux) | `~/.claude.json` | **verified** 2026-10-18 on WSL2 |
| Claude Code (macOS) | `~/.claude.json` | TODO(verify) |
| Claude Desktop (macOS) | `~/Library/Application Support/Claude/claude_desktop_config.json` | TODO(verify) |
| Claude Desktop (Linux) | `~/.config/Claude/claude_desktop_config.json` | TODO(verify); no official Linux build |
| Cursor | `~/.cursor/mcp.json` | TODO(verify); `~/.cursor` exists, no `mcp.json` yet |
| VS Code (Linux) | `~/.config/Code/User/mcp.json` | TODO(verify) |
| VS Code (WSL/remote) | `~/.vscode-server/data/User/mcp.json` | TODO(verify); dir exists, no file yet |
| VS Code (macOS) | `~/Library/Application Support/Code/User/mcp.json` | TODO(verify) |

**Next step:** the owner adds one server in each app and follows [the runbook](../runbooks/verify-config-paths.md).
