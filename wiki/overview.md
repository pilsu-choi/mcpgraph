---
type: Project
title: Project overview
description: What mcpgraph does, its scope, the hard rules it follows and where it stands.
tags: [mcpgraph, overview, scope]
timestamp: 2026-10-18T07:25:00Z
---

# Project overview

## What it is

A tiny EDR for AI agents ([v2 spec](decisions/v2-transition.md)). It has two jobs:

1. **Visibility (posture).** Which AI apps are installed, which MCP servers they use, and which
   processes, credentials and data sources those servers can reach.
2. **Runtime control.** Judge each MCP tool call (through a stdio proxy) and each coding-agent
   shell command (through a hook) with one policy engine: allow, deny or ask. Every decision is
   audited and appears in the graph.

## Context

This is a portfolio project for a backend/platform engineering role at an AI endpoint security
company. Code quality, clear design decisions and tests matter more than feature count. The owner
is learning Go, so the code is deliberately plain, idiomatic Go with comments on idioms that
are not obvious.

## Scope (v2 MVP)

- macOS and Linux.
- Discovery for Claude Desktop, Claude Code, Cursor and VS Code
  ([config collector](components/config-collector.md), [process collector](components/process-collector.md)).
- Graph, [posture rules R1–R7](rules/index.md), CLI report, JSON and Mermaid export.
- YAML policy engine, stdio MCP proxy, Claude Code hook, append-only audit log, Decision nodes.
- The DROP TABLE demo, with PostgreSQL.

Stretch goals are an event sink with a collector ([`Sink`](components/sink.md)) and the daemon
([archived](decisions/archive-v1-daemon.md)). Out of scope: eBPF, Windows, remote HTTP MCP behind
the proxy, and fleet UI.

## Hard rules

1. Read-only by default. Only `install` / `uninstall` edit AI-app configs, with backups, a diff and `--yes`.
2. It never stores or prints secret values. Tool-call args and commands are redacted too ([redact at collection](decisions/redact-at-collection.md)).
3. No network egress by default; the local API listens on 127.0.0.1 only ([loopback-only API](decisions/loopback-only-api.md)).
4. Fail closed on `tools/call`, and pass everything else through untouched.
5. No cgo ([pure Go](decisions/pure-go-no-cgo.md)).
6. External formats are not guessed: config paths, the MCP wire format and hook formats are checked
   first ([research](research/external-formats.md), [path issue](issues/config-path-verification.md)).

## Status

The v1 milestones (posture) are done, and the [v2 transition](milestones/v2-transition.md) brought
them in line with the new spec. The [policy engine](milestones/v2-m5-policy-engine.md) (milestone 5) and the [audit log](milestones/v2-m6-audit-log.md) (milestone 6) and the [MCP proxy](milestones/v2-m7-proxy.md) (milestone 7) and the [install / uninstall](milestones/2026-10-18-v2-m8-install.md) commands (milestone 8) are done. Next is milestone 9, the Claude Code hook, with Codex CLI hooks if their format can be verified, then milestone 10 (demo GIF) and milestone 11 (collector). See the
[milestones](milestones/index.md). The code is on GitHub at
[pilsu-choi/mcpgraph](https://github.com/pilsu-choi/mcpgraph) (`dev` branch).
