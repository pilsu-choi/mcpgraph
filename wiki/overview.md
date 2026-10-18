---
type: Project
title: Project overview
description: What mcpgraph does, its scope, the hard rules it follows and where it stands.
tags: [mcpgraph, overview, scope]
timestamp: 2026-10-18T06:50:00Z
---

# Project overview

## Question it answers

> Which AI apps are installed, which MCP servers do they use, and which processes, credentials and
> data sources can those servers reach?

## Context

This is a portfolio project for a backend/platform engineering role at an AI endpoint security
company. Code quality, clear design decisions and tests matter more than feature count. The owner
is learning Go, so the code is deliberately plain, idiomatic Go with comments on idioms that
are not obvious.

## Scope (MVP)

- macOS and Linux.
- Config parsers for Claude Desktop, Claude Code, Cursor and VS Code ([config collector](components/config-collector.md)).
- Process discovery ([process collector](components/process-collector.md)).
- Graph, [rules R1–R6](rules/index.md), CLI report, JSON and Mermaid export.
- A daemon with fsnotify, SQLite snapshots and diffs ([daemon](components/daemon.md)).
- A read-only HTTP API on localhost ([api](components/api.md)).

Out of scope: eBPF and kernel hooks, Windows, and a central server. For the central server only the
[`Sink`](components/sink.md) interface exists.

## Hard rules

1. Read-only: it never writes outside its own data directory and never signals processes.
2. It never stores or prints secret values ([redact at collection](decisions/redact-at-collection.md)).
3. No network egress; the only listener is 127.0.0.1 ([loopback-only API](decisions/loopback-only-api.md)).
4. No cgo ([pure Go](decisions/pure-go-no-cgo.md)).
5. Config paths are not guessed: unverified ones are marked `TODO(verify)` ([issue](issues/config-path-verification.md)).

## Status

All nine [milestones](milestones/index.md) are done. On top of that, the
[docs and visuals](milestones/m10-docs-visuals.md) work added the README diagrams, captures, run
logs and this wiki. The code is on GitHub at
[pilsu-choi/mcpgraph](https://github.com/pilsu-choi/mcpgraph) (`dev` branch).
