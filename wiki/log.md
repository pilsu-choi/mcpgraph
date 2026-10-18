---
type: Log
title: Work log
description: Chronological history of work on mcpgraph, newest last.
tags: [log, history]
timestamp: 2026-10-18T06:45:00Z
---

# Work log

Times are UTC. Each entry links to the page with the details.

## 2026-10-18

- **05:55** Environment set up: Go 1.27.2 installed to `~/.local/go`, golangci-lint v2.15.0 and
  GoReleaser to `~/go/bin`. `make` and `gcc` are not installed (no passwordless sudo), so `-race`
  cannot run locally. See [dev environment](runbooks/dev-environment.md) and
  [issue](issues/race-detector-local.md).
- **06:04** [m1 skeleton](milestones/m1-skeleton.md) committed (`d1c0170`).
- **06:10** [m2 config collector](milestones/m2-config-collector.md) committed (`3626db6`). Only
  `~/.claude.json` verified on the real machine ([issue](issues/config-path-verification.md)).
- **06:11** [m3 redaction](milestones/m3-redaction.md) committed (`f4e2568`).
- **06:13** [m4 process collector](milestones/m4-process-collector.md) committed (`64f8e58`).
  Two real-world bugs found: [npm process title](lessons/npm-process-title.md) and
  [shell `-c` false positive](lessons/shell-c-false-positive.md).
- **06:16** [m5 builder + exports](milestones/m5-builder-exports.md) committed (`8bae004`).
  Mermaid checked with the mermaid 11 parser, since no browser was available.
- **06:18** [m6 rules + report](milestones/m6-rules-report.md) committed (`6af1a60`). The real
  machine shows one R6 finding ([issue](issues/plugin-servers-invisible.md)).
- **06:21** [m7 daemon](milestones/m7-daemon.md) committed (`4635de7`). The diff is logged about
  100 ms after a config edit.
- **06:23** [m8 API](milestones/m8-api.md) committed (`92ece4b`, amended for a gofmt fix).
- **06:24** [m9 release + README](milestones/m9-release-readme.md) committed (`0a474ab`).
- **06:30–06:43** [Docs and visuals](milestones/m10-docs-visuals.md):
  [termsvg](components/termsvg.md) capture tool, `scripts/demo.sh` and `make demo`, run logs in
  `docs/logs/`, README rebuilt with 9 Mermaid diagrams (all checked with the mermaid parser and
  rendered in headless Chromium), and this wiki. The module path was renamed to
  `github.com/pilsu-choi/mcpgraph`. Lessons:
  [SVG whitespace](lessons/svg-whitespace.md), [Mermaid strikethrough](lessons/mermaid-tilde.md).
- **06:43** Commits `2c12b0d` (module path, inventory ordering) and `06d1446` (docs) on a new
  `dev` branch.
- **06:44** Public GitHub repository `pilsu-choi/mcpgraph` created through the REST API; `dev`
  pushed ([runbook](runbooks/github-repo.md)).
- **06:45** Found that `scripts/demo.sh` reported a dirty tree: `> file` truncates
  `00-environment.log` before `git describe` runs. Fixed (`6eb59da`), and logs regenerated at a clean commit.
