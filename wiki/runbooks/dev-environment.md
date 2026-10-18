---
type: Runbook
title: Development environment
description: Toolchain locations on the owner's WSL2 machine and what is missing.
tags: [runbook]
timestamp: 2026-10-18T06:50:00Z
---

# Development environment

| Tool | Where | Notes |
| --- | --- | --- |
| Go 1.27.2 | `~/.local/go/bin` | add to `PATH` |
| golangci-lint 2.15.0 | `~/go/bin` | `go install .../v2/cmd/golangci-lint` |
| GoReleaser v2 | `~/go/bin` | `go install github.com/goreleaser/goreleaser/v2` |
| make, gcc | **missing** | `sudo apt install -y build-essential` |

```sh
export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH
```

Without gcc, `make test` runs without `-race` and prints a warning ([issue](../issues/race-detector-local.md)).
