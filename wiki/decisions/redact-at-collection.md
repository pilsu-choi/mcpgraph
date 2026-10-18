---
type: Design Decision
title: Redact secrets at collection time
description: Raw config values never leave the config package; every stored string is masked first.
tags: [decision, security, secrets]
timestamp: 2026-10-18T06:11:37Z
---

# Redact secrets at collection time

## Decision
- The parsed raw type `serverConfig` is **unexported**, so it cannot leave `internal/collector/config`.
- `derive.go` is the only reader of raw values. Args, URL and command go through
  [`redact`](../components/redact.md).
- Env values are dropped. Only key names are kept.
- A detected secret becomes a `Credential` node with kind, key name and location, never the value.
- Process command lines are redacted with the same function, so masked config args and masked
  process args still compare equal, and [RUNS_AS matching](../components/graph-builder.md) works.

## Why
If masking happened at output time, every new output format would be a chance to leak. Masking once
at the edge makes every output safe by construction: table, JSON, Mermaid, SQLite and API.

## Verification
- `internal/scan/leak_test.go` renders all formats, including the raw SQLite and WAL files, and
  looks for each fake secret and any 8-character fragment of it.
- `scripts/docs.sh` greps the generated `docs/logs` and `docs/images` for the fake secrets.
- Mask format: `****` plus at most the last 4 characters, and only for values of 12 characters or more.
