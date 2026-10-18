---
type: Go Package
title: internal/sink and internal/central
description: Opt-in shipper of audit decisions (sink) and the minimal collector that stores events from several machines (central).
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/sink
tags: [component, go, sink, collector, v2, m11]
timestamp: 2026-10-18T08:32:00Z
---

# `internal/sink` and `internal/central`

Supersedes the old no-op [sink](sink.md). Why it works this way:
[ship from the audit log](../decisions/2026-10-18-ship-from-audit-log.md).

## `internal/sink` (client)

| File | Role |
| --- | --- |
| `sink.go` | wire types: `Event` (a `model.Decision` plus `id`) and `Batch` (`host`, `events`) |
| `config.go` | `sink.json` (`url`, `token`, `host`, mode 0600), `CheckURL` (http/https only, warns on plain http to a non-loopback host), `LoadConfig`, `SaveConfig`, `RemoveConfig` |
| `ship.go` | `Ship(ctx, dataDir)`, `Pending`, `Flusher` |

- `Ship` is the one function to call. It does nothing, and makes no request, when `sink.json` is
  missing. It takes an exclusive non-blocking `flock` on `sink.lock` (skips if held), reads complete
  lines of `audit.jsonl` from the byte offset in `sink.offset`, re-redacts summary and reason, POSTs up to 500
  events per request (`Authorization: Bearer` when a token is set, 2 s timeout) and saves the offset only
  after a 2xx. The event ID is the SHA-256 of the raw audit line.
- `Flusher` (nil when the sink is off) wraps the proxy's recorder: after each recorded decision it
  starts one background `Ship` (never two at once); `Close` does a last flush with a 2 s deadline.
  Errors are logged as warnings and never change what the proxy does.

## `internal/central` (collector)

- `Store`: `events.jsonl` (0600, O_APPEND) loaded into memory at start; dedup by event ID; `Add`,
  `Query` (reuses `audit.Filter`), `Hosts`.
- `Handler(store, token)`: `POST /v1/events` (validates, 4 MiB body cap -> 413, constant-time bearer
  check, re-redaction), `GET /v1/events` (`host`, `verdict`, `source`, `since`, `limit`),
  `GET /v1/hosts`, `GET /healthz` (open). `Serve` shuts down gracefully when the context is cancelled.
- `Query` is the client call behind `mcpgraph events --collector`.

CLI: `mcpgraph sink set|off|status|flush`, `mcpgraph collector --listen 127.0.0.1:7600 --data DIR
[--token T]`. A non-loopback `--listen` without `--token` prints a warning.
