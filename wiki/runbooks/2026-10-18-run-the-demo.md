---
type: Runbook
title: Run the DROP TABLE demo
description: Prerequisites, commands, expected output and troubleshooting for make demo and the README animation.
resource: https://github.com/pilsu-choi/mcpgraph/blob/dev/demo
tags: [runbook, demo]
timestamp: 2026-10-18T08:31:00Z
---

# Run the DROP TABLE demo

## Prerequisites

- Docker with `docker compose` v2 (tested: Docker 28, Compose 2.34).
- Node.js with `npx` (path A runs `@modelcontextprotocol/server-postgres@0.6.2`; first run downloads it).
- Go on `PATH` (`export PATH=$HOME/.local/go/bin:$PATH`), and `python3`.
- Path B only: the `claude` CLI, logged in, and an `mcpgraph` that has `hook claude-code` (milestone 9).

No `make` on the owner's machine: run the scripts directly.

## Commands

```sh
make demo                      # or: demo/run.sh   (both paths, then teardown)
SKIP_HOOK=1 demo/run.sh        # path A only
make demo-down                 # or: demo/down.sh   (after an interrupted run)
DEMO_PG_PORT=55500 demo/run.sh # if 55442 is taken
```

Each path can run alone: `MCPGRAPH=<binary> DEMO_DATA=<dir> demo/proxy-demo.sh` (or `hook-demo.sh`).
The data dir is throwaway, so your real audit log is not touched.

## Expected output (path A)

```
  initialized: example-servers/postgres
  tools: query
> tools/call query {"sql":"SELECT count(*) FROM users"}
  allowed -> [   {     "count": "6"   } ]
> tools/call query {"sql":"DROP TABLE users"}
  DENIED  rule=block-destructive-sql -> mcpgraph: this tool call was blocked by policy rule ...
DECISIONS (2)
... ALLOW ... SELECT count(*) FROM users
... DENY  block-destructive-sql ... DROP TABLE users
users table still exists.
```

Path B ends with `mcpgraph events --verdict deny` showing a `hook` decision and the same
`users table still exists.`

## Recording the README animation

```sh
demo/record.sh                                  # docs/images/demo-draft.svg (placeholder hook part)
demo/hook-demo.sh > hook.txt                    # a real hook run, once the hook works
HOOK_TRANSCRIPT=hook.txt demo/record.sh --final # docs/images/demo.svg
```

Transcript format for `termsvg -animate`: plain lines; `#@ 1.5` pauses 1.5 s before the next line;
`$ ` lines are typed. The loop is the transcript length plus 4 s.

## Troubleshooting

- **`docker daemon not reachable` on WSL2:** in Docker Desktop, Settings, Resources, WSL
  integration, enable your distro, then reopen the terminal. `docker info` must work.
- **Port already in use:** another container may hold 55442; set `DEMO_PG_PORT`.
- **`no answer for request 1`:** `npx` could not fetch the server (offline, or a proxy). Run the
  `npx -y @modelcontextprotocol/server-postgres@0.6.2 --help` once to see the error.
- **npm prints `deprecated`:** expected, the package is unsupported but pinned and read-only.
- **Path B: `no 'hook claude-code' yet`:** milestone 9 is not in your build.
- **Leftover container:** `make demo-down`.
