---
type: Lesson
title: Shell scripts that mention mcp
description: bash -c scripts that merely contain 'mcp' were flagged MCP-like.
tags: [lesson]
timestamp: 2026-10-18T06:13:45Z
---

# Shell scripts that mention "mcp"

**Symptom:** `/bin/bash -c "... go run ./cmd/mcpgraph ..."` and the `go run` process were flagged
as MCP-like. They would have become R6 shadow-server findings.

**Fix:** for `sh|bash|zsh|dash|fish -c SCRIPT`, look only at the first word of SCRIPT
(`sh -c playwright-mcp` still counts). Skip arguments containing `mcpgraph`, and skip tools such as
grep and ps (test pid 500 in `process_test.go`).

**Takeaway:** keep heuristics narrow and document their limits in the README.
