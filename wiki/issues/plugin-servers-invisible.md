---
type: Issue
title: Plugin-provided MCP servers are not in configs
description: On the real machine, Claude Code's Playwright MCP comes from a plugin and is flagged R6.
tags: [issue, open]
timestamp: 2026-10-18T06:50:00Z
---

# Plugin-provided MCP servers are not in configs

On the owner's machine, `npm exec @playwright/mcp@latest` (child of `claude`) is reported as
**R6 shadow server**. It is started by a Claude Code plugin, not declared in `~/.claude.json`.
mcpgraph is correct that the server is undeclared in any scanned config, but the finding is noisy.

Options:

- Parse Claude Code plugin manifests under `~/.claude/plugins` (needs path verification).
- Add an allowlist.

See `docs/logs/06-scan-real-machine.log`.
