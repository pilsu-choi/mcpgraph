---
type: Index
title: Lessons
description: Bugs and surprises found during development.
tags: [lesson]
timestamp: 2026-10-18T07:25:00Z
---

# Lessons

Bugs found while building, and what they taught.

- [npm rewrites its process title](npm-process-title.md): A running npx MCP server did not get RUNS_AS because its argv was one string.
- [Shell scripts that mention mcp](shell-c-false-positive.md): bash -c scripts that merely contain 'mcp' were flagged MCP-like.
- [SVG captures lost column alignment](svg-whitespace.md): Chromium collapsed runs of spaces inside SVG text despite xml:space=preserve.
- [Two tildes in a Mermaid label become strikethrough](mermaid-tilde.md): Mermaid parsed '~/.ssh ... ~/.aws' as markdown strikethrough and rendered an error box.
- [Trailing-quote trim and prose SQL](sql-quote-and-prose.md): Two policy SQL classifier bugs found by tricky-case table tests.
- [Redacting twice masked the mask](redaction-idempotent.md): Defence-in-depth re-redaction needed redaction to be idempotent.
- [Go's JSON struct decoding is case-insensitive](json-case-insensitive.md): A "Method" key could hide a tools/call from a struct decoder.
- [redact.Text is wrong for raw JSON lines](2026-10-18-json-diff-redaction.md): A config diff needed redaction inside decoded string literals, not on whole lines.
