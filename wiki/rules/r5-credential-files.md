---
type: Risk Rule
title: R5: Credential files inside an accessible path
description: `~/.aws/credentials`, `~/.netrc`, `~/.pgpass`, `~/.ssh/id_*` (not `.pub`), or `<root>/.env` existing inside a filesystem root. Only stat/glob, never opened.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/posture/r5_credential_files.go
tags: [rule, r5, high]
timestamp: 2026-10-18T06:18:47Z
---

# R5: Credential files inside an accessible path

- **Severity:** high
- **Detects:** `~/.aws/credentials`, `~/.netrc`, `~/.pgpass`, `~/.ssh/id_*` (not `.pub`), or `<root>/.env` existing inside a filesystem root. Only stat/glob, never opened.
- **Rationale (shown in the finding):** Names the exact file one prompt injection could exfiltrate.
- **Test cases:** home → 2 (aws + ssh key), .ssh → 1, project with .env → 1, clean project → 0.

Source: [`internal/posture/r5_credential_files.go`](https://github.com/pilsu-choi/mcpgraph/blob/dev/internal/posture/r5_credential_files.go). Rule interface and ordering: [rules component](../components/posture.md).
