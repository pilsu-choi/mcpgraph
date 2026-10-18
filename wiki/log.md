---
type: Log
title: Work log
description: Chronological history of work on mcpgraph, newest last.
tags: [log, history]
timestamp: 2026-10-18T07:25:00Z
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
- **06:47** CI enabled for `dev` (`24846a9`). First GitHub Actions run
  [39105184003](https://github.com/pilsu-choi/mcpgraph/actions/runs/39105184003) passed:
  golangci-lint, `go test -race ./...` and the cgo-free build. This is the first `-race` run of the
  project ([issue](issues/race-detector-local.md)).
- **06:50** The owner moved the project to AGENTS.md v2: a tiny EDR for AI agents with posture plus
  runtime control ([decision](decisions/v2-transition.md)). Owner decisions:
  - archive the v1 daemon ([decision](decisions/archive-v1-daemon.md));
  - fsnotify and sqlite stay approved for m12;
  - keep plain HTTP as R7 ([decision](decisions/plain-http-as-r7.md));
  - the demo DB is PostgreSQL.
- **06:52** Branch `archive/v1-daemon` pushed at `a33015b`.
- **06:55** A research subagent checked the MCP stdio format and the Claude Code hook contract
  against official docs ([research](research/external-formats.md)). Key findings:
  - MCP `2026-07-28` drops `initialize`.
  - The spec prefers `isError` results over JSON-RPC errors for refusals the model should see.
  - Hooks fail open unless `onFailure: block` is set.

  Three issues opened: [denial format](issues/proxy-denial-format.md),
  [MCP revision](issues/mcp-revision-per-client.md), [hooks fail open](issues/hook-fails-open.md).
- **07:05** [v2 transition](milestones/v2-transition.md):
  - the daemon, store, API and watcher were removed;
  - `rules` became `posture`, with R4 "not proxied" and R7 "plain http";
  - Decision model types and CLI stubs were added;
  - Makefile targets `bench` and `docs` were added;
  - `scripts/docs.sh` was created, the README updated, and the logs regenerated.
- **07:15** [v2 m5 policy engine](milestones/v2-m5-policy-engine.md): `internal/policy`,
  `policies/default.yaml` (embedded) and `redact.Text`.
  - 98 tricky-case table tests pass.
  - The engine is fail closed on error, panic or timeout.
  - One decision takes about 13 µs against the 5 ms budget.
  - Two classifier bugs were found by the tests ([lesson](lessons/sql-quote-and-prose.md)).
  - `Action.Cwd` was added ([decision](decisions/action-cwd.md)).
  - The README has a new *Runtime policy* section.
- **07:20** CI run [39106767302](https://github.com/pilsu-choi/mcpgraph/actions/runs/39106767302)
  passed on `f75d409` (m5), including `-race`.
- **07:22** Owner decisions:
  - proxy denials are `isError` results ([decision](decisions/proxy-denial-iserror.md), which
    closes the [issue](issues/proxy-denial-format.md));
  - deny rules come before ask rules ([decision](decisions/deny-before-ask.md)).

  `default.yaml` was reordered, and AGENTS.md §15 updated.
- **07:25** [v2 m6 audit log](milestones/v2-m6-audit-log.md):
  - [`internal/audit`](components/audit.md), with `mcpgraph events`;
  - Decision nodes linked with `DECIDED_ON`;
  - `tools/replay`;
  - the replay acceptance test and an extended leak test.

  Redaction made idempotent ([lesson](lessons/redaction-idempotent.md)). README has a new *Audit log
  and events* section, and docs logs 13–17 were added.

- **07:35** [v2 m7 MCP proxy](milestones/v2-m7-proxy.md):
  - [`internal/proxy`](components/proxy.md) and `mcpgraph proxy -- <server>`;
  - transcript acceptance test with a real SDK 1.30.0 capture, plus edge tests;
  - JSON key-case bypass closed ([lesson](lessons/json-case-insensitive.md));
  - the [revision issue](issues/mcp-revision-per-client.md) partly answered (SDK speaks 2025-11-25).

  `tools/fakemcp` and docs logs 18–19 were added. README has a new *MCP proxy* section.
- **07:45** CI run 39108220573 failed on `c14a0dc`: the 2 MiB proxy test hit its 1 s policy deadline
  under `-race` and failed closed. The test now uses a 30 s deadline ([milestone notes](milestones/v2-m7-proxy.md#notes)).
