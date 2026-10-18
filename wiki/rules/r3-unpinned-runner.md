---
type: Risk Rule
title: R3: Unpinned package runner
description: `npx`/`bunx`/`pnpx`/`uvx` with a package spec that has no exact version (`pkg`, `pkg@latest`, `pkg@^1`, `pkg@1.x`).
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/rules/r3_unpinned_runner.go
tags: [rule, r3, medium]
timestamp: 2026-10-18T06:18:47Z
---

# R3: Unpinned package runner

- **Severity:** medium
- **Detects:** `npx`/`bunx`/`pnpx`/`uvx` with a package spec that has no exact version (`pkg`, `pkg@latest`, `pkg@^1`, `pkg@1.x`).
- **Rationale (shown in the finding):** Every start fetches the newest publish, so a compromised or typosquatted release runs right away.
- **Test cases:** 14 cases: scoped packages, `--package=`, `--from`, `==` and `@` pins, non-runner commands.

Source: [`internal/rules/r3_unpinned_runner.go`](https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/rules/r3_unpinned_runner.go). Rule interface and ordering: [rules component](../components/rules.md).
