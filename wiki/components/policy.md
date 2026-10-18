---
type: Go Package
title: internal/policy
description: Runtime policy engine: YAML loading, matchers (SQL classifier, shell commands, recursive delete, path globs, unknown tool), Engine.Decide and the fail-closed Judge.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/policy
tags: [component, go, policy, v2]
timestamp: 2026-10-18T07:15:00Z
---

# `internal/policy`

Both runtime interception paths (proxy, hook) depend on this package through one interface:

```go
type Engine interface {
    Decide(ctx context.Context, a Action) (model.Decision, error)
}
```

| File | Role |
| --- | --- |
| `policy.go` | YAML types (`Policy`, `Rule`, `When`, `Match`), `Parse` with `KnownFields(true)`, compilation and validation |
| `engine.go` | `Action`, `Options`, `RuleEngine.Decide` (first match wins), `Summary`, fail-closed `Judge` |
| `sql.go` | SQL classifier: drop_table, drop_database (and schema), truncate, delete_without_where with tautology detection |
| `shell.go` | Splits a command line into simple commands. Handles quotes, escapes, comments, `&& \|\| ; \| &`, wrappers (sudo, env, xargs, ...) and nested `sh -c` up to depth 3 |
| `delete.go` | `rm -r` targets outside the project directory |
| `paths.go` | Path globs with `**`, home expansion, path-like words from commands and arguments |

The shipped policy is [`policies/default.yaml`](https://github.com/pilsu-choi/mcpgraph/blob/dev/policies/default.yaml),
embedded with `//go:embed` in package `policies`. Its rules are listed under
[runtime policy rules](../rules/runtime-policy.md).

## How a decision is made

1. `newInput` prepares the action once for all rules:
   - the text (`Action.Text`, or every string inside `Args` when the text is empty),
   - its shell segments,
   - the SQL classes found,
   - the resolved path-like words.
2. Rules run in order. A rule applies when its `when` filters hold (sources; tools, which are
   anchored, case-insensitive regexes). It fires when **any** matcher fires.
3. The first rule that fires decides. When none fires, the policy `default` applies (allow, unless
   the policy says otherwise).
4. The `Decision` carries `Summary(text)`: `redact.Text`, then a cap of 200 runes. Raw `Args`
   never reach it.

## Fail closed

`Judge(ctx, engine, action, timeout)` runs `Decide` in a goroutine. The result channel is buffered,
so a late goroutine never blocks. An error, panic or timeout becomes a deny with rule ID
`mcpgraph-fail-closed`. The proxy (m7) will call `Judge`, never `Decide` directly.

## Speed

About 13 µs and 87 allocations per decision, against a budget of 5 ms. `TestDecideIsFast` enforces
the budget on every test run, and `make bench` reports the number.
