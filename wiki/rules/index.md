---
type: Index
title: Rules
description: Risk rules R1–R6 evaluated on every graph.
tags: [rule]
timestamp: 2026-10-18T06:18:47Z
---

# Rules

| ID | Rule | Severity |
| --- | --- | --- |
| [R1](r1-plaintext-secret.md) | Plaintext secret in server config | high |
| [R2](r2-broad-filesystem.md) | Filesystem server rooted at $HOME or / | high |
| [R3](r3-unpinned-runner.md) | Unpinned package runner | medium |
| [R4](r4-plain-http.md) | Remote MCP over plain http:// | medium |
| [R5](r5-credential-files.md) | Credential files inside an accessible path | high |
| [R6](r6-shadow-server.md) | Undeclared MCP-like process | medium |

`mcpgraph scan` and `mcpgraph report` exit with code 2 when any finding is high.
