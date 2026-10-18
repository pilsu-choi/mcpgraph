---
type: Milestone
title: "v2 m7: stdio MCP proxy"
description: mcpgraph proxy -- <server> judges tools/call with the policy engine, answers denials as isError results, forwards everything else byte for byte, fails closed and never hangs; acceptance test replays a real SDK capture.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/proxy
tags: [milestone, v2, m7, proxy]
timestamp: 2026-10-18T07:35:00Z
---

# v2 m7: stdio MCP proxy

**Done when** (AGENTS.md): a recorded transcript replays through the proxy; allowed calls are
forwarded unchanged, denied `tools/call` requests are answered with the rule ID, and non-`tools/call`
messages are byte-identical.

`TestTranscriptReplay` in `internal/proxy/proxy_test.go` does this for three transcripts:

- `captured-sdk-1.30.0.jsonl`: a **real** session between the official MCP TypeScript SDK 1.30.0
  client and server. It negotiated protocol `2025-11-25`, and its IDs start at 0.
- `legacy-2025-11-25.jsonl` and `modern-2026-07-28.jsonl`: hand-written from the spec (and marked
  as such). They cover a server-to-client request, a cancel for a denied request, odd spacing and
  unicode, an unknown tool, a `Method`/`method` bypass attempt and a secret read.

A mutation check (turning off `tools/call` detection) makes the test fail, so the test does catch
a proxy that stops judging.

## Delivered

- [`internal/proxy`](../components/proxy.md) and `mcpgraph proxy [--server] [--app] [--policy]
  [--decision-timeout] -- cmd args...`.
- Denials and asks are `isError` results with `_meta` `mcpgraph/rule_id` and `mcpgraph/verdict`
  ([decision](../decisions/proxy-denial-iserror.md)).
- A broken `--policy` file makes every tool call fail closed instead of stopping the proxy.
- Edge tests: fail closed on error and timeout, malformed input, a 2 MiB message, CRLF, batches,
  unknown tools before and after `tools/list`, server exit, no secret in denials or audit,
  child exit code, missing binary.
- `tools/fakemcp` and docs logs 18–19 plus `18-proxy.svg`. README section *MCP proxy*.
- The real SDK client through the proxy against the real SDK server (scratch run, not in CI):
  the client accepts the `isError` result and `_meta`; `DROP TABLE`, `WHERE 1=1` and an unlisted
  tool were blocked, and four decisions were written.

## TODO(verify)

- `_meta` key names `mcpgraph/rule_id` and `mcpgraph/verdict`: they follow the spec's
  `prefix/name` rule, but the prefix is not a reverse-DNS name, and no spec page says what clients
  do with unknown `_meta` keys beyond ignoring them.
- Which revision each AI app speaks ([issue](../issues/mcp-revision-per-client.md)): only the SDK
  is known so far.

## Go concepts

- **`map[string]json.RawMessage`** keeps each field's raw bytes and matches keys exactly. Struct
  decoding matches field names case-insensitively, which would let `"Method"` hide a call
  ([lesson](../lessons/json-case-insensitive.md)).
- **One goroutine per direction** with a mutex-guarded writer (`syncWriter`), because both the
  server and the proxy itself (denials) write to the client.
- **`io.Pipe`** in tests gives a synchronous in-memory stdin/stdout, so a test can send a line and
  wait for the answer.
- **`exec.CommandContext` with `Cancel` and `WaitDelay`**: on cancel the child gets SIGTERM, and
  after 5s Go stops waiting for its pipes, so the proxy cannot hang on a stuck server.
- **`TestMain` re-exec**: the test binary runs itself as a fake server when an env var is set,
  which tests `RunCommand` without any external program.
