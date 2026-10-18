---
type: Issue
title: Race detector cannot run locally
description: No C compiler on the dev machine, so go test -race only runs in CI.
tags: [issue, open]
timestamp: 2026-10-18T06:45:00Z
---

# Race detector cannot run locally

`go test -race` needs cgo, and there is no `gcc` (or `make`) on the WSL2 machine. Passwordless
sudo is not available, so it was not installed.

- The Makefile detects gcc and falls back to `go test ./...` with a warning.
- CI (`ubuntu-latest`) runs `go test -race ./...`.

**Fix:** `sudo apt install -y build-essential`.
