---
type: Runbook
title: Verify config paths
description: How to confirm a candidate path before removing its TODO(verify).
tags: [runbook]
timestamp: 2026-10-18T06:45:00Z
---

# Verify config paths

AGENTS.md hard rule 5: paths are candidates until confirmed on a real machine.

1. Install or open the app and add one MCP server through its UI.
2. Find the file it wrote, for example with `ls -la` on the candidate in `internal/collector/config/paths.go`.
3. Check that the JSON shape matches the parser's fixture.
4. Set `verified` to `true`, replace `TODO(verify)` with "Verified <date> on <OS>", and add a line
   to the verification log in the `Candidates` doc comment.
5. Record it in [log.md](../log.md) and update [the issue](../issues/config-path-verification.md).
