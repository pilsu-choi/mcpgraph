---
type: Milestone
title: v2 m6: Audit log, events and Decision nodes
description: Append-only redacted JSONL audit log, mcpgraph events with filters, Decision nodes with DECIDED_ON edges, replay tool; plus the owner decisions on isError denials and deny-before-ask.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/audit
tags: [milestone, v2, m6, audit]
timestamp: 2026-10-18T07:25:00Z
---

# v2 m6: Audit log, events and Decision nodes

**Done when** (AGENTS.md): a test replays decisions and the graph shows `DECIDED_ON` edges.
`internal/scan/replay_test.go` does this. It judges six actions (hook and proxy, including fake
secrets) with the shipped policy, appends them to an audit log, scans the fixture home and checks
each Decision node's verdict, rule and `DECIDED_ON` target. One action, against a server not in any
config, must have no edge. A second test checks that Decision IDs are the same on every scan.

## Delivered

- [`internal/audit`](../components/audit.md): `Open`/`Append`/`Read`/`Filter`/`Collector`.
- The graph builder's `linkDecisions`; a *RECENT DECISIONS* section in the inventory;
  `report.Events` and `EventsJSONL`.
- CLI:
  - `mcpgraph events` with `--since 24h|7d|RFC3339`, `--verdict`, `--source`, `--agent`,
    `--tool`, `--rule`, `-n` and `-f table|jsonl`;
  - global `--data-dir` and `--decisions N` (default 50, 0 to turn it off).
- `model.Decision.App` and `policy.Action.App`.
- `tools/replay` and `testdata/actions/demo.jsonl`. The docs logs 13–17 and `14-events.svg` come
  from them.
- The leak test now covers the audit file, both `events` formats, and Decision nodes in every
  graph format.
- Redaction is idempotent ([lesson](../lessons/redaction-idempotent.md)).
- Owner decisions: [isError denials](../decisions/proxy-denial-iserror.md) and
  [deny before ask](../decisions/deny-before-ask.md). Both are in AGENTS.md §15.

## Notes

- Report formatting was kept simple, as AGENTS.md allows for m6 ("cut 6's report formatting").
- Linking relies on names. A proxy decision names its server (`Agent`) and app (`App`), and the
  m8 install will put both into the proxy's arguments.
- No external format is involved, so there is no `TODO(verify)`.

## Go concepts

- **`os.O_APPEND` + one `Write` per record** gives safe concurrent appends across processes;
  `sync.Mutex` serialises writers within one process.
- **`bufio.Scanner` limits:** the default maximum token is 64 KiB, and `sc.Buffer(buf, max)`
  raises it. Without that, a long line stops the scan with `ErrTooLong`.
- **`errors.Is(err, os.ErrNotExist)`** treats a missing log as empty instead of failing.
- **`defer l.Close()`** in tests, and `t.TempDir()` for isolated files.
- **Persistent cobra flags** (`--data-dir`) are shared by every subcommand through one options
  struct, with no globals.
