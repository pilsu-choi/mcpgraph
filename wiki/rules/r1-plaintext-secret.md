---
type: Risk Rule
title: R1: Plaintext secret in server config
description: An `inline` Credential reachable over USES_SECRET: a literal token in env, args, headers or the URL.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/rules/r1_plaintext_secret.go
tags: [rule, r1, high]
timestamp: 2026-10-18T06:18:47Z
---

# R1: Plaintext secret in server config

- **Severity:** high
- **Detects:** An `inline` Credential reachable over USES_SECRET: a literal token in env, args, headers or the URL.
- **Rationale (shown in the finding):** Config files are synced, backed up and read by other tools.
- **Test cases:** inline credential → 1 finding; env reference, credential file path, or no credentials → 0.

Source: [`internal/rules/r1_plaintext_secret.go`](https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/rules/r1_plaintext_secret.go). Rule interface and ordering: [rules component](../components/rules.md).
