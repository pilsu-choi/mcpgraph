---
type: Decision
title: Ship events from the audit log, not inline
description: The sink reads audit.jsonl from a saved byte offset and sends batches with content-hash IDs; it is opt-in via sink.json.
tags: [decision, sink, collector, v2, m11, hard-rule-3]
timestamp: 2026-10-18T08:34:00Z
---

# Ship events from the audit log, not inline

**Decision.** The proxy (and later the hook) never sends a decision over the network itself. They
append to the audit log as before. A shipper reads the log from a byte offset kept in `sink.offset`,
POSTs batches, and moves the offset only after a 2xx. Each event has an ID, the SHA-256 of its raw
audit line, so the collector drops repeats.

**Why.**

- The audit log is already the source of truth and is written before anything else happens. Network
  trouble can delay shipping but never loses or changes a decision, and never delays the agent.
- At-least-once delivery with IDs is easy to reason about: a crash, a 500 or a lost reply just means
  the same lines are sent again and ignored.
- The hook is a new process per tool call, so it cannot keep a queue in memory. A file offset works
  for every process; an exclusive `flock` on `sink.lock` makes sure one shipper runs at a time.
- Only complete lines are shipped, so a line another process is still writing waits.

**Hard rule 3.** Nothing leaves the machine unless `<data-dir>/sink.json` exists, which only
`mcpgraph sink set <url>` creates (mode 0600). `sink off` removes it. Summary and reason are redacted
again before sending, and again by the collector. `sink set` warns about plain `http://` to a
non-loopback host (the R7 posture); the token is never printed (`status` masks it).

**Collector.** Loopback by default; a non-loopback `--listen` is explicit and warns without
`--token`. Dedup state is rebuilt from `events.jsonl` at start.

**Rejected.** Sending inline from the proxy (blocks or loses events when the network is down);
a queue in memory (lost on exit, not shared with the hook); a new interface with several sinks (one
HTTP sink is enough, so the old snapshot `Sink` interface was removed).
