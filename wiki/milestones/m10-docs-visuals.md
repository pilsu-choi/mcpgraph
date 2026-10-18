---
type: Milestone
title: Docs, visuals, run logs and wiki
description: README diagrams and captures, reproducible run logs, the OKF wiki, and the GitHub repository.
tags: [milestone, docs]
timestamp: 2026-10-18T06:50:00Z
---

# Docs, visuals, run logs and wiki

The owner asked for after m9: a richer README with Mermaid diagrams and run captures, log files
that show real runs, and a wiki of the work in OKF format, then a GitHub repository with the work
pushed to `dev`.

## Delivered
- [`tools/termsvg`](../components/termsvg.md): turns terminal output into SVG terminal windows.
- `scripts/demo.sh` / `make demo`: runs the fixtures, the real machine, the daemon with two config
  edits, the API (including 405/403), the refused bind, tests, lint and the release build. Writes
  `docs/logs/00…13-*.log` and `docs/images/*.svg`, then fails if any fake secret appears in them.
- README: 9 Mermaid diagrams (problem, findings pie, fixture graph, real-machine graph, architecture,
  graph model, scan sequence, redaction flow, daemon sequence) and 6 captures.
- This wiki ([index](../index.md)), plus AGENTS.md §14 requiring future work to be logged here.
- Module path changed to `github.com/pilsu-choi/mcpgraph`.
- Inventory now sorts AI apps by name. The ACCESSES column was widened to 70 characters.

## Verification
- Every README Mermaid block parsed with mermaid 11.4.1 (jsdom). All 9 were also rendered in
  headless Chromium (Playwright's cached build) and inspected. This found one bug
  ([lesson](../lessons/mermaid-tilde.md)).
- Every SVG capture was rasterised and inspected. This found the whitespace bug
  ([lesson](../lessons/svg-whitespace.md)).
- `golangci-lint run`: 0 issues. `go test ./...`: all packages ok.
