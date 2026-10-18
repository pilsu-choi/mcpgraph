---
type: Index
title: Rules
description: Posture rules R1–R7 evaluated on every graph.
tags: [rule]
timestamp: 2026-10-18T07:05:00Z
---

# Posture rules

| ID | Rule | Severity |
| --- | --- | --- |
| [R1](r1-plaintext-secret.md) | Plaintext secret in server config | high |
| [R2](r2-broad-filesystem.md) | Filesystem server rooted at $HOME or / | high |
| [R3](r3-unpinned-runner.md) | Unpinned package runner | medium |
| [R4](r4-not-proxied.md) | stdio MCP server not routed through the proxy (v2) | medium |
| [R5](r5-credential-files.md) | Credential files inside an accessible path | high |
| [R6](r6-shadow-server.md) | Undeclared MCP-like process | medium |
| [R7](r7-plain-http.md) | Remote MCP over plain http:// (R4 in v1) | medium |

These are *posture* rules: they judge configuration. *Runtime* policy rules such as
`block-destructive-sql` judge single actions and live in `policies/default.yaml` (milestone 5).

`mcpgraph scan` and `mcpgraph report` exit with code 2 when any finding is high.
