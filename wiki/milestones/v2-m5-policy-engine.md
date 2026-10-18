---
type: Milestone
title: v2 m5: Policy engine
description: YAML policy, SQL classifier, shell splitting, path globs, recursive-delete and unknown-tool matchers, Engine.Decide, fail-closed Judge, default policy, tricky-case table tests, benchmark.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/policy
tags: [milestone, v2, m5, policy]
timestamp: 2026-10-18T07:15:00Z
---

# v2 m5: Policy engine

**Done when** (AGENTS.md): table tests for every runtime rule pass, and the benchmark shows under
5 ms per decision. Both are met: 98 table cases pass, and a decision takes about 13 µs.

## Delivered

- [`internal/policy`](../components/policy.md): `Parse`/`LoadFile`, `New`, `RuleEngine.Decide`,
  `Judge`, `Summary`.
- [`policies/default.yaml`](../rules/runtime-policy.md), embedded in the binary.
- `redact.Text` for free text ([redact](../components/redact.md)).
- Dependency `gopkg.in/yaml.v3` v3.0.1. AGENTS.md v2 §9 lists it, so it was already approved.
- `scripts/docs.sh` writes `11-bench.log`, `12-policy-tests.log` and `11-bench.svg`.
- The README has a new *Runtime policy* section.

## Tests

| Test | Covers |
| --- | --- |
| `default_test.go` | The five shipped rules, through the real YAML: positive, negative and tricky cases |
| `engine_test.go` | Rejected policies (unknown keys, bad verdicts, regexes, globs, duplicate IDs, rules without matchers), first match wins, `when` filters, cancelled context, `Judge` fail-closed on error/panic/hang, no secret in any Decision |
| `shell_test.go` | Splitter (quotes, escapes, comments, `2>&1`, wrappers, nested shells, depth bound), globs, tautologies |
| `bench_test.go` | `BenchmarkDecide`, plus `TestDecideIsFast`, which enforces the 5 ms budget in `make test` |

Two classifier bugs were found while writing the tests ([lesson](../lessons/sql-quote-and-prose.md)).

## Deviations and open points

- `Action` has an extra `Cwd` field ([decision](../decisions/action-cwd.md)).
- `ask-unknown-tool` needs a tool inventory, which scans do not have yet. With no inventory the rule
  never fires. The proxy (m7) can build one from `tools/list` responses.
- Rule order follows the spec, so `rm -rf ~/.ssh` asks instead of denying
  ([note](../rules/runtime-policy.md)).
- No external format is involved in m5, so there is nothing to mark `TODO(verify)`.

## Go concepts

- **`//go:embed`** puts `default.yaml` into the binary as a `[]byte`. It needs the blank import of
  `embed`.
- **Compile once, use many times:** `regexp.MustCompile` at package level, and the policy's regexes
  inside `New`. `*regexp.Regexp` is safe to use from many goroutines.
- **Goroutine + buffered channel + `select` with `ctx.Done()`** gives a timeout that works even if
  the callee ignores the context. `recover()` in a deferred function turns a panic into an error.
- **Compile-time interface check:** `var _ Engine = (*RuleEngine)(nil)`.
- **`b.Loop()`** (Go 1.24+) in benchmarks; table-driven subtests with `t.Run`.
