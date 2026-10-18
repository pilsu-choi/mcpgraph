---
type: Milestone
title: m3: Redaction
description: `internal/redact` detection (prefixes, secret-named keys, entropy, references) and masking.
resource: https://github.com/pilsu-choi/mcpgraph/commit/f4e2568
tags: [milestone, m3]
timestamp: 2026-10-18T06:11:37Z
---

# m3: Redaction

**Commit:** [`f4e2568`](https://github.com/pilsu-choi/mcpgraph/commit/f4e2568)

## Delivered
`internal/redact` detection (prefixes, secret-named keys, entropy, references) and masking. End-to-end leak test.

## Acceptance check
`TestNoSecretInAnyOutput` runs the fixtures through every output format and checks for raw values and 8-character fragments.

## Go concepts used
- Table-driven tests with `t.Run`
- `net/url` parsing and re-encoding
- Shannon entropy with `math.Log2`
- `strings.Cut` for key=value
