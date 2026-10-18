---
type: Runbook
title: Regenerate docs logs and captures
description: How docs/logs and docs/images are produced and how to refresh them.
tags: [runbook]
timestamp: 2026-10-18T07:25:00Z
---

# Regenerate docs logs and captures

```sh
make docs        # = scripts/docs.sh   (was make demo / scripts/demo.sh in v1)
```

What it does:

1. Builds `mcpgraph` with `CGO_ENABLED=0`, plus `tools/termsvg`.
2. Builds a temporary home from `testdata/configs`, plus fake `~/.aws/credentials` and
   `~/.ssh/id_ed25519` so that R5 fires.
3. Runs scan, report and export on it. The temporary path is shown as `/home/dev` and the hostname
   as `devbox`.
4. Runs `mcpgraph scan` on the real machine. **That log is not edited**: it contains the real
   hostname, user paths and process list.
5. Replays `testdata/actions/demo.jsonl` with `tools/replay` into a temporary audit log, then runs
   `events`, a scan with decisions, and a Mermaid export of the Decision subgraph (logs 13–17).
6. Runs tests, lint and the GoReleaser snapshot build.
7. Renders the SVG captures, then greps everything for fake secrets and fails on a match.

To view a capture as PNG locally: open the SVG in a browser.
