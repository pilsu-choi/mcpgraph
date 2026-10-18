---
type: Runbook
title: Regenerate demo logs and captures
description: How docs/logs and docs/images are produced and how to refresh them.
tags: [runbook]
timestamp: 2026-10-18T06:45:00Z
---

# Regenerate demo logs and captures

```sh
make demo        # = scripts/demo.sh
```

What it does:

1. Builds both binaries with `CGO_ENABLED=0`, plus `tools/termsvg`.
2. Builds a temporary home from `testdata/configs`, plus fake `~/.aws/credentials` and
   `~/.ssh/id_ed25519` so that R5 fires.
3. Runs scan, report and export on it. The temporary path is shown as `/home/dev` and the hostname
   as `devbox`.
4. Runs `mcpgraph scan` on the real machine. **That log is not edited**: it contains the real
   hostname, user paths and process list.
5. Starts `mcpgraphd` on port 17878 (override with `MCPGRAPH_DEMO_PORT`), edits the Cursor config
   twice, calls the API, and stops it with SIGINT.
6. Runs tests, lint and the GoReleaser snapshot build.
7. Renders the SVG captures, then greps everything for fake secrets and fails on a match.

To view a capture as PNG locally: open the SVG in a browser.
