---
type: Risk Rule
title: R2: Filesystem server rooted at $HOME or /
description: A filesystem DataSource whose cleaned path equals the home directory or `/`.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/posture/r2_broad_filesystem.go
tags: [rule, r2, high]
timestamp: 2026-10-18T06:18:47Z
---

# R2: Filesystem server rooted at $HOME or /

- **Severity:** high
- **Detects:** A filesystem DataSource whose cleaned path equals the home directory or `/`.
- **Rationale (shown in the finding):** One prompt injection can read SSH keys, cloud credentials and browser data.
- **Test cases:** home, home/, / → 1; project directory, `/home/u2` (prefix trap), database at `/` → 0.

Source: [`internal/posture/r2_broad_filesystem.go`](https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/posture/r2_broad_filesystem.go). Rule interface and ordering: [rules component](../components/posture.md).
