---
type: Lesson
title: npm rewrites its process title
description: A running npx MCP server did not get RUNS_AS because its argv was one string.
tags: [lesson]
timestamp: 2026-10-18T06:13:45Z
---

# npm rewrites its process title

**Symptom:** the declared server `npx @playwright/mcp@latest` showed no PID, even though
`npm exec @playwright/mcp@latest` (pid 1654) was running.

**Cause:** npm sets `process.title`, so `/proc/<pid>/cmdline` holds a single string with spaces.
gopsutil's `CmdlineSlice` returned one element, and argument matching failed.

**Fix:** `normalizeArgs` splits a single-element argv that contains spaces (`TestNormalizeArgs`).

**Takeaway:** test process logic against a real process table, not only fixtures.
