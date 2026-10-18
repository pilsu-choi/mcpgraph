---
type: Decision
title: Move to AGENTS.md v2: posture plus runtime control
description: The owner replaced the spec with v2, which turns mcpgraph from a read-only inventory tool into a tiny EDR for AI agents.
tags: [decision, v2, scope]
timestamp: 2026-10-18T07:05:00Z
---

# Move to AGENTS.md v2: posture plus runtime control

On 2026-10-18 the owner replaced [AGENTS.md](../../AGENTS.md) with v2. mcpgraph now has two jobs:

1. **Visibility (posture).** What v1 built: AI apps, MCP servers, processes, credentials, data
   sources, and posture rules on the graph.
2. **Runtime control.** Catch an action before it runs (an MCP `tools/call` through a stdio proxy,
   or a coding-agent shell command through a pre-tool-use hook), judge it with one policy engine
   (`allow` / `deny` / `ask`), and write every decision to an append-only audit log and the graph.

The success scenario: Claude Code tries `DROP TABLE users`, the rule `block-destructive-sql` denies
it, and the decision shows up in the audit log and the graph.

## What changed in the spec

| Area | v1 | v2 |
| --- | --- | --- |
| Hard rule 1 | Read-only, always | Read-only by default; only `install` / `uninstall` edit configs, with backups, a diff and `--yes` |
| New hard rules | — | Fail closed on `tools/call`, pass everything else through; do not guess *any* external format (paths, MCP wire format, hook formats) |
| Binaries | `mcpgraph` + `mcpgraphd` | One binary with `scan report export proxy hook install uninstall events` |
| Packages | `rules/`, `store/`, `api/`, `daemon/`, `collector/watch/` | `posture/`, plus `policy/ proxy/ hook/ install/ audit/` |
| R4 | Plain `http://` to a remote host | Not routed through the proxy ([plain HTTP kept as R7](plain-http-as-r7.md)) |
| Daemon | Milestone 7 | Stretch milestone 12 ([archived](archive-v1-daemon.md)) |
| `make demo` | Regenerate docs | The DROP TABLE demo; docs moved to `make docs` |

## Owner decisions

Recorded in AGENTS.md §15:

- Archive the v1 daemon, store, API and watcher on a branch ([details](archive-v1-daemon.md)).
- `fsnotify` and `modernc.org/sqlite` stay approved for milestone 12.
- Keep the plain-HTTP rule as R7 ([details](plain-http-as-r7.md)).
- The demo database is PostgreSQL.
- Before milestones 7 and 9, verify the MCP wire format and the Claude Code hook format
  ([research](../research/external-formats.md)).

Transition work: [v2 transition milestone](../milestones/v2-transition.md).
