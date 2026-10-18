---
type: Index
title: mcpgraph wiki
description: Entry point of the mcpgraph knowledge bundle: what the project is, how it is built, and why.
tags: [mcpgraph, index]
timestamp: 2026-10-18T07:05:00Z
---

# mcpgraph wiki

This wiki is an Open Knowledge Format (OKF v0.1)
bundle: plain Markdown files with YAML frontmatter, one concept per file, linked with ordinary
relative links. Each directory has an `index.md`, and [log.md](log.md) holds the chronological
history of work on the project.

**mcpgraph** is a tiny EDR for AI agents, written in Go. It maps the AI apps on a developer laptop,
their MCP servers, and what those servers can reach (posture). It also judges what agents are
about to do, allowing, denying or asking first (runtime control). Start
with the [project overview](overview.md).

| Section | What is there |
| --- | --- |
| [overview.md](overview.md) | Problem, scope, hard rules, current status |
| [architecture/](architecture/index.md) | Pipeline, graph model, concurrency |
| [decisions/](decisions/index.md) | Design decisions and their reasons |
| [components/](components/index.md) | One page per package |
| [rules/](rules/index.md) | Posture rules R1–R7 |
| [research/](research/index.md) | External formats checked against official docs |
| [milestones/](milestones/index.md) | What each milestone delivered, how it was checked, Go concepts used |
| [runbooks/](runbooks/index.md) | How to set up, run the demo, verify paths, add an app |
| [lessons/](lessons/index.md) | Bugs found while building, and what they taught |
| [issues/](issues/index.md) | Open questions and known gaps |
| [log.md](log.md) | Chronological work log |

## Conventions for this wiki

- Every page has `type` (required), plus `title`, `description`, `tags` and `timestamp` (UTC, ISO 8601).
- Pages about code carry `resource:`, a link to the source on GitHub.
- New work is recorded in [log.md](log.md) and in the page it changes ([AGENTS.md](../AGENTS.md) §14).
