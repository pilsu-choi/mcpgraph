# mcpgraph

[![ci](https://github.com/pilsu-choi/mcpgraph/actions/workflows/ci.yml/badge.svg)](https://github.com/pilsu-choi/mcpgraph/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/go-1.27-00ADD8)
![cgo](https://img.shields.io/badge/cgo-disabled-success)
![platforms](https://img.shields.io/badge/platforms-darwin%20%7C%20linux-lightgrey)

A tiny EDR for AI agents, written in Go. One binary, two jobs:

1. **Visibility (posture).** Which AI apps are installed, which MCP servers do they use, and which
   processes, credentials and data sources can those servers reach? *(done)*
2. **Runtime control.** Catch what an agent is about to do (an MCP tool call, a shell command from a
   coding agent), check it against a policy, and **allow, deny or ask** before it runs. Every
   decision goes to an audit log and into the graph. *(done on both paths: the MCP proxy and the
   coding-agent hook, plus an opt-in central collector)*

<p align="center">
  <img src="docs/images/demo.svg" alt="Claude Code is asked to clean up the database; mcpgraph blocks DROP TABLE users and logs the decision" width="100%">
</p>

<sub><b>The demo (<code>make demo</code>), recorded from a real run.</b> Claude Code (2.1.302) is asked to drop the
<code>users</code> table of a throwaway PostgreSQL. It runs <code>psql -c 'DROP TABLE users CASCADE;'</code>, the
<code>mcpgraph hook</code> matches rule <code>block-destructive-sql</code>, the command never runs, Claude reports why, and
the decision is in the audit log. Then the same policy stops a <code>tools/call</code> to a real Postgres MCP server
through <code>mcpgraph proxy</code>. Claude's long answer is trimmed (marked …); nothing else is edited.
See <a href="#demo">Demo</a>.</sub>

## Contents

- [The problem](#the-problem)
- [Demo](#demo)
- [Quick start](#quick-start)
- [What it finds](#what-it-finds)
- [Runtime policy](#runtime-policy)
- [MCP proxy](#mcp-proxy)
- [Install and uninstall](#install-and-uninstall)
- [Coding-agent hook](#coding-agent-hook)
- [Audit log and events](#audit-log-and-events)
- [Central collector (opt-in)](#central-collector-opt-in)
- [Architecture](#architecture)
- [Design decisions](#design-decisions)
- [Roadmap](#roadmap)
- [Testing and quality gates](#testing-and-quality-gates)
- [Run logs](#run-logs)
- [Limitations](#limitations) · [Future work](#future-work)

## The problem

AI coding tools (Claude Desktop, Claude Code, Cursor, VS Code) now launch
[MCP](https://modelcontextprotocol.io) servers. These are small programs that give the model tools
such as "read files", "query Postgres" or "post to Slack". Each app keeps its own config file in its
own format and location, so on a typical laptop nobody has the full picture:

```mermaid
flowchart LR
  subgraph laptop["Developer laptop"]
    direction LR
    CD["Claude Desktop<br/>claude_desktop_config.json"]
    CC["Claude Code<br/>~/.claude.json"]
    CU["Cursor<br/>~/.cursor/mcp.json"]
    VS["VS Code<br/>User/mcp.json (JSONC)"]
    CD --> S1["filesystem server<br/>rooted at ~"]
    CC --> S2["slack server<br/>token in plain text"]
    CU --> S3["remote server<br/>over http://"]
    VS --> S4["npx -y pkg<br/>no version"]
    P["MCP process<br/>not in any config"]
  end
  S1 -.reads.-> K["SSH private keys<br/>AWS credentials"]
  classDef risk fill:#fee2e2,stroke:#b91c1c,color:#000
  class S1,S2,S3,S4,P,K risk
```

- API tokens are pasted in plain text into JSON config files that get synced and backed up.
- A filesystem server is given `~`, which includes `~/.ssh` and `~/.aws`, so one prompt injection can read them.
- `npx -y some-mcp-server` downloads and runs the latest publish on every start.
- Some MCP processes run without being declared in any config at all.

mcpgraph puts all of this into one graph and runs risk rules on it.

## Demo

```sh
make demo        # Docker + Node (npx) + Claude Code logged in; about a minute
make demo-down   # clean up after an interrupted run
SKIP_HOOK=1 demo/run.sh   # only the MCP proxy path: no Claude Code, no account needed
```

`demo/run.sh` builds mcpgraph into a temp dir and starts PostgreSQL 17 in Docker on
`127.0.0.1:55442` (demo-only credentials, `DEMO_PG_PORT` to change the port) with a small `users` /
`orders` schema. Then it runs both interception paths:

1. **Hook path** ([`demo/hook-demo.sh`](demo/hook-demo.sh)). A throwaway project gets a
   project-local `.claude/settings.json` with `mcpgraph hook claude-code` as its PreToolUse hook
   (`onFailure: block`); your own `~/.claude/settings.json` is never touched. A `psql` wrapper on
   `PATH` talks to the container. One `claude -p` run asks Claude Code to drop the `users` table.
2. **Proxy path** ([`demo/proxy-demo.sh`](demo/proxy-demo.sh)). The official
   `@modelcontextprotocol/server-postgres@0.6.2` runs behind `mcpgraph proxy`; an MCP client sends
   `SELECT count(*)` (allowed and forwarded) and `DROP TABLE users` (answered by mcpgraph with an
   `isError` result; the server never sees it).

Both end with `mcpgraph events` and a `select count(*) from users` that proves the table is still
there; the database is removed with `docker compose down -v` on exit.

**About the prompt.** Asked only to "clean up the database: drop the users table", Claude Code
looked at the data with read-only queries (which the hook allowed) and then stopped to ask, without
running the DROP. That is the agent's own judgement, not mcpgraph. The demo prompt therefore says the
database is a throwaway with a backup and not to ask first, so that the agent actually attempts the
DROP and the guard rail is what stops it. When blocked, Claude Code did not try to get around the
hook and told the user how to proceed.

The animation is an SVG made by [`tools/termsvg -animate`](tools/termsvg) from the transcript
([`demo/record.sh`](demo/record.sh)): CSS keyframes only, no scripts, so it plays in the README.

## Quick start

```sh
make build                     # CGO_ENABLED=0 go build ./cmd/...
./mcpgraph scan                # inventory + findings; exit code 2 if any finding is high
./mcpgraph report -f json      # findings only, for CI
./mcpgraph export -f mermaid   # graph as a Mermaid flowchart
./mcpgraph events --verdict deny --since 24h   # runtime decisions from the audit log
make docs                      # regenerate docs/logs and docs/images from real runs
```

Useful flags: `--home DIR` scans another home directory (for example the fixtures), and
`--no-processes` skips process discovery.

## What it finds

### Rules

| ID | Detects | Severity | Why it matters |
| --- | --- | --- | --- |
| R1 | Plaintext secret in an MCP server's env, args, headers or URL | high | Config files are synced, backed up and read by other tools |
| R2 | Filesystem MCP server rooted at `$HOME` or `/` | high | One prompt injection can read SSH keys and cloud credentials |
| R3 | Unpinned package runner (`npx -y pkg`, `uvx pkg`, `pkg@latest`) | medium | A compromised publish runs on the next start |
| R4 | stdio MCP server not routed through the mcpgraph proxy | medium | Its tool calls skip the policy check and the audit log |
| R5 | Credential files (`~/.aws/credentials`, `~/.ssh/id_*`, `.env`, ...) inside a path an MCP server can access | high | Names the exact files that are exposed |
| R6 | MCP-like process not declared in any config (shadow server) | medium | Its tools and permissions are invisible to review |
| R7 | Remote MCP over plain `http://` to a non-loopback host | medium | Tool calls and auth headers travel unencrypted |

R7 was R4 in the first version. R4 now means "not behind the proxy", and the plain-HTTP check kept
its logic under a new ID. Until `mcpgraph install` has been run (or a server is wrapped by hand), R4 fires for
every stdio server.

`scan` and `report` exit with code **2** when any finding is high, so they can gate a CI job.

Findings on the fixture home, by rule:

```mermaid
pie showData
  title Fixture findings by rule (9 high, 10 medium)
  "R1 plaintext secret" : 6
  "R5 credential file exposed" : 2
  "R2 broad filesystem root" : 1
  "R3 unpinned runner" : 3
  "R4 not proxied" : 6
  "R7 plain http" : 1
```

<p align="center">
  <img src="docs/images/02-report-fixtures.svg" alt="mcpgraph report output" width="100%">
</p>

### The graph

This is the Claude Desktop part of the fixture graph, exported with `mcpgraph export -f mermaid`
(the full export is in [`docs/logs/04-export-fixtures-mermaid.log`](docs/logs/04-export-fixtures-mermaid.log)):

```mermaid
flowchart LR
  classDef AIApp fill:#dbeafe,stroke:#1d4ed8,color:#000
  classDef MCPServer fill:#ede9fe,stroke:#6d28d9,color:#000
  classDef Credential fill:#fee2e2,stroke:#b91c1c,color:#000
  classDef DataSource fill:#fef3c7,stroke:#b45309,color:#000
  n_AIApp_c344979fd5fe["AIApp<br/>Claude Desktop"]:::AIApp
  n_Credential_b06380fcc6e0["Credential<br/>inline: GITHUB_PERSONAL_ACCESS_TOKEN"]:::Credential
  n_Credential_d6bb33066a64["Credential<br/>inline: args[2]"]:::Credential
  n_DataSource_21abe27a441f["DataSource<br/>database: postgresql://db.internal:5432/prod"]:::DataSource
  n_DataSource_3360c9452281["DataSource<br/>filesystem: /home/dev"]:::DataSource
  n_DataSource_e6d4e38df883["DataSource<br/>saas: github"]:::DataSource
  n_MCPServer_5f8b681e933b["MCPServer<br/>Claude Desktop / postgres"]:::MCPServer
  n_MCPServer_e2d7e891ca14["MCPServer<br/>Claude Desktop / github"]:::MCPServer
  n_MCPServer_ea8e707dfcb3["MCPServer<br/>Claude Desktop / filesystem"]:::MCPServer
  n_AIApp_c344979fd5fe -->|CONFIGURES| n_MCPServer_5f8b681e933b
  n_AIApp_c344979fd5fe -->|CONFIGURES| n_MCPServer_e2d7e891ca14
  n_AIApp_c344979fd5fe -->|CONFIGURES| n_MCPServer_ea8e707dfcb3
  n_MCPServer_5f8b681e933b -->|ACCESSES| n_DataSource_21abe27a441f
  n_MCPServer_5f8b681e933b -->|USES_SECRET| n_Credential_d6bb33066a64
  n_MCPServer_e2d7e891ca14 -->|ACCESSES| n_DataSource_e6d4e38df883
  n_MCPServer_e2d7e891ca14 -->|USES_SECRET| n_Credential_b06380fcc6e0
  n_MCPServer_ea8e707dfcb3 -->|ACCESSES| n_DataSource_3360c9452281
```

And from a real machine, where Claude Code runs a Playwright MCP server. Process discovery links the
declared server to the running `npm exec` process and follows its children:

```mermaid
flowchart LR
  classDef AIApp fill:#dbeafe,stroke:#1d4ed8,color:#000000
  classDef MCPServer fill:#ede9fe,stroke:#6d28d9,color:#000000
  classDef Process fill:#dcfce7,stroke:#15803d,color:#000000
  n_AIApp_16cfd094c6a7["AIApp<br/>Claude Code"]:::AIApp
  n_MCPServer_05bc20b7fcc5["MCPServer<br/>Claude Code / playwright"]:::MCPServer
  n_Process_d080c1c01431["Process<br/>claude (1588)"]:::Process
  n_Process_67e9439f809f["Process<br/>npm exec @playw (1654)"]:::Process
  n_Process_e36828d32d16["Process<br/>sh (1698)"]:::Process
  n_Process_eaf31313d171["Process<br/>MainThread (1699)"]:::Process
  n_AIApp_16cfd094c6a7 -->|CONFIGURES| n_MCPServer_05bc20b7fcc5
  n_MCPServer_05bc20b7fcc5 -->|RUNS_AS| n_Process_67e9439f809f
  n_Process_67e9439f809f -->|CHILD_OF| n_Process_d080c1c01431
  n_Process_e36828d32d16 -->|CHILD_OF| n_Process_67e9439f809f
  n_Process_eaf31313d171 -->|CHILD_OF| n_Process_e36828d32d16
```

## Runtime policy

Runtime control judges one action at a time: an MCP `tools/call` (through the proxy, milestone 7) or
a coding-agent tool use such as a Bash command (through the hook, milestone 9). Both paths build the
same `policy.Action` and call the same `policy.Engine`, so each rule is written once. The engine and
the shipped policy are done (milestone 5), and so are the [MCP proxy](#mcp-proxy) (milestone 7) and
the [coding-agent hook](#coding-agent-hook) (milestone 9).

```mermaid
flowchart LR
  P["mcpgraph proxy<br/>tools/call"] --> A["policy.Action<br/>source · agent · tool · args · text · cwd"]
  H["mcpgraph hook<br/>Bash, Read, ..."] --> A
  A --> J["policy.Judge<br/>timeout · recover"]
  J --> E["RuleEngine.Decide<br/>rules in order, first match wins"]
  E --> D["model.Decision<br/>verdict · rule_id · reason · summary*"]
  J -. "error, panic or timeout" .-> F["deny<br/>mcpgraph-fail-closed"]
  classDef deny fill:#fee2e2,stroke:#b91c1c,color:#000
  class F deny
```

<sub>* the summary is the action text after <code>redact.Text</code>, capped at 200 characters.
Raw arguments never reach a Decision.</sub>

The shipped policy, [`policies/default.yaml`](policies/default.yaml), is embedded in the binary:

| Rule | Matches | Verdict |
| --- | --- | --- |
| `block-destructive-sql` | `DROP TABLE`, `DROP DATABASE/SCHEMA`, `TRUNCATE`, `DELETE` without a limiting `WHERE` | deny |
| `block-secret-read` | any path under `~/.ssh/`, `~/.aws/credentials`, `.env` / `.env.*` (not `.env.example`) | deny |
| `ask-recursive-delete` | `rm -r` / `-rf` / `--recursive` on a path that is not strictly inside the project directory | ask |
| `ask-git-force-push` | `git push --force`, `-f`, `--force-with-lease`, `+refspec` | ask |
| `ask-unknown-tool` | an MCP tool the last scan did not see (proxy only) | ask |

Deny rules come first, so when an action matches both kinds the deny wins: `rm -rf ~/.ssh` is
denied, not asked. When the proxy blocks or asks, it answers `tools/call` with a
normal result marked `isError: true`. The text names the rule and the reason, and `_meta` carries
the rule ID. The MCP spec says clients *should* show tool errors to the model but only *may* show
JSON-RPC errors, and the agent has to see why it was stopped.

A rule looks like this:

```yaml
- id: block-destructive-sql
  verdict: deny
  reason: Destructive SQL is blocked ...
  match:
    sql: [drop_table, drop_database, truncate, delete_without_where]
```

Matchers are `sql` (a small statement classifier), `command` (regex per simple command), `text`
(regex on the whole text), `recursive_delete`, `paths` / `paths_except` (globs with `**`) and
`unknown_tool`. `when` narrows a rule by `sources` (proxy/hook) and `tools`. Unknown YAML keys are
errors, so a typo cannot quietly switch a rule off.

**Tricky cases are tested.** There are 98 table cases in
[`docs/logs/12-policy-tests.log`](docs/logs/12-policy-tests.log). Some examples:

| Input | Result |
| --- | --- |
| `psql -c "dRoP tAbLe users"` | deny (mixed case) |
| `DROP/**/TABLE users`, `DROP -- x⏎TABLE users` | deny (comments between keywords) |
| `DELETE FROM users WHERE 1=1`, `... WHERE id = 5 OR 'a'='a'` | deny (tautological WHERE) |
| `DELETE FROM users -- WHERE id = 5` | deny (WHERE only in a comment) |
| `psql -c "DELETE FROM users" && echo "where id=3"` | deny (WHERE belongs to another command) |
| `bash -c "psql -c 'DROP TABLE users'"` | deny (nested shell) |
| `DELETE FROM jobs WHERE finished_at IS NULL` | allow |
| `truncate -s 0 app.log`, `git log --grep "delete from"` | allow (not SQL) |
| `make clean && rm -rf ~/.cache`, `rm -rf "$TARGET"` | ask (chained; unresolved variable) |
| `rm -rf build dist/*` in the project | allow |
| `git push origin main && rm -f x.lock` | allow (the `-f` belongs to `rm`) |

**Fail closed.** `policy.Judge` runs `Decide` in a goroutine with a deadline. An error, a panic or
a timeout becomes a denial with rule ID `mcpgraph-fail-closed`, and the caller is never blocked past
the deadline (hard rule 4).

**Fast.** Regexes and globs are compiled once, when the policy loads. One decision takes about
**13 µs** ([`docs/logs/11-bench.log`](docs/logs/11-bench.log)), far under the 5 ms budget, and a
test enforces that budget on every `make test`.

<p align="center">
  <img src="docs/images/11-bench.svg" alt="make bench" width="80%">
</p>

## MCP proxy

`mcpgraph proxy` sits between an AI app and a stdio MCP server. The app starts mcpgraph instead of
the server, and mcpgraph starts the real server as its child:

```sh
mcpgraph proxy --server postgres --app "Claude Desktop" -- npx -y @modelcontextprotocol/server-postgres postgres://...
```

You do not have to edit configs by hand: [`mcpgraph install`](#install-and-uninstall) wraps every
stdio server this way, with backups, and `uninstall` undoes it.

```mermaid
sequenceDiagram
  participant C as AI app (MCP client)
  participant P as mcpgraph proxy
  participant E as policy.Judge
  participant S as MCP server (child)
  participant L as audit.jsonl
  C->>P: initialize, tools/list, notifications ...
  P->>S: forwarded byte for byte
  S->>P: tools/list result
  Note over P: remembers the tool names
  P->>C: forwarded byte for byte
  C->>P: tools/call query "SELECT 1"
  P->>E: Action{tool, args}
  E-->>P: allow
  P->>L: decision
  P->>S: forwarded byte for byte
  C->>P: tools/call query "DROP TABLE users"
  P->>E: Action{tool, args}
  E-->>P: deny (block-destructive-sql)
  P->>L: decision
  P->>C: result isError: true, reason + rule ID
  Note over S: never sees the call
```

**Transparent by default.** Only `tools/call` is judged. Every other message, in both directions,
is copied as the exact bytes that arrived: whitespace, key order, unicode escapes and `\r\n` line
ends included. A test sends a 2 MiB message and a CRLF message and compares bytes. Server stderr is
passed through, and the server's exit code becomes mcpgraph's exit code.

**Denied and asked calls** get a normal tool result with `isError: true`. The text names the rule
and the reason; `_meta` carries `mcpgraph/rule_id` and `mcpgraph/verdict`. The real server never
sees the call. A later `notifications/cancelled` for that request is dropped, since the server never
knew about it. The proxy cannot show an approval prompt, so `ask` blocks too, and the text says how
to approve (run the action yourself, or add an allow rule with `--policy`).

**Fail closed, never hang (hard rule 4).** Each `tools/call` is judged by `policy.Judge` with a
deadline (`--decision-timeout`, default 2s). A broken `--policy` file does not stop the proxy;
every tool call is denied with `mcpgraph-fail-closed` and the reason, while everything else keeps
working. A malformed `tools/call` (bad `params`) is denied, not forwarded. When the server exits, the
proxy exits with it.

**No parser bypass.** Go's `encoding/json` matches struct fields case-insensitively, so a message
with both `"method":"ping"` and `"Method":"tools/call"` could look like a ping to a struct decoder
while a server reads it differently. The proxy reads messages as `map[string]json.RawMessage` and
looks up exact keys only. Batches (JSON arrays, removed from MCP in 2025-06-18) are forwarded
unless they contain a `tools/call` that is not allowed; then the whole batch is answered locally.

**Unknown tools.** The proxy learns the server's tools from its `tools/list` responses. A call to a
tool that was not listed gets `ask-unknown-tool`. Before the first `tools/list`, every tool counts
as known, so a client that never lists tools is not blocked.

**Tested on real traffic.** [`testdata/jsonrpc/captured-sdk-1.30.0.jsonl`](testdata/jsonrpc/captured-sdk-1.30.0.jsonl)
is a session captured between the official MCP TypeScript SDK 1.30.0 client and server (it
negotiated protocol `2025-11-25`). The acceptance test replays it, and two hand-written transcripts
for the `2025-11-25` and `2026-07-28` revisions, through the proxy: allowed calls must reach the
server unchanged, denied ones must come back with the rule ID, and every other message must be
byte-identical. Running the SDK client through the proxy against the SDK server showed the client
accepts the `isError` result and its `_meta`.

`make docs` pipes the client side of that capture through `mcpgraph proxy` wrapping
[`tools/fakemcp`](tools/fakemcp/main.go), a scripted server that runs nothing
([`docs/logs/18-proxy-session.log`](docs/logs/18-proxy-session.log)):

<p align="center">
  <img src="docs/images/18-proxy.svg" alt="mcpgraph proxy session" width="100%">
</p>

## Install and uninstall

`mcpgraph install` rewrites the MCP configs of Claude Desktop, Claude Code (`~/.claude.json`,
including per-project servers), Cursor and VS Code, so that each stdio server starts through the
[proxy](#mcp-proxy). Without `--yes` it is a dry run: it prints the plan and a redacted diff, and
writes nothing.

```sh
mcpgraph install                                  # dry run: plan + diff, nothing written
mcpgraph install --yes                            # back up, then rewrite
mcpgraph install --app Cursor --server github --yes   # only one app / server
mcpgraph install --policy ./my-policy.yaml --yes  # proxies use this policy file
mcpgraph uninstall --yes                          # undo, newest install first
```

Every stdio server becomes `/abs/path/mcpgraph proxy --server NAME --app APP -- <original command
and args>`. Remote `http` / `sse` servers and servers that are already proxied are listed as "left
alone". This is the real dry-run diff for Claude Desktop (paths shortened):

```diff
--- /home/dev/.config/Claude/claude_desktop_config.json (now)
+++ /home/dev/.config/Claude/claude_desktop_config.json (after install)
@@ -2,10 +2,10 @@
   "mcpServers": {
     "filesystem": {
-      "command": "npx",
+      "command": "/usr/local/bin/mcpgraph",
-      "args": ["-y", "@modelcontextprotocol/server-filesystem", "~"]
+      "args": ["proxy", "--server", "filesystem", "--app", "Claude Desktop", "--", "npx", "-y", "@modelcontextprotocol/server-filesystem", "~"]
     },
```

<p align="center">
  <img src="docs/images/20-install.svg" alt="mcpgraph install dry run" width="100%">
</p>

```mermaid
flowchart LR
  subgraph I["install --yes"]
    direction LR
    PL["plan<br/>find stdio servers"] --> BK["back up each file<br/>0600 in a 0700 dir"]
    BK --> WR["atomic write<br/>temp file, fsync, rename"]
    WR --> MF["manifest.json<br/>paths, hashes, offsets"]
  end
  subgraph U["uninstall --yes"]
    direction LR
    CH{"file exactly<br/>as install left it?"}
    CH -- yes --> RS["restore the backup<br/>byte for byte"]
    CH -- "no, the app changed it" --> UW["unwrap in place<br/>keep the app's changes"]
    RS --> DEL["delete backups"]
    UW --> DEL
  end
  MF -.-> CH
```

**Only `command` and `args` change.** A small span-preserving JSON/JSONC parser finds the byte range
of each `"command"` and `"args"` value and replaces just those ranges. Indentation, key order,
comments and the rest of a multi-megabyte `~/.claude.json` stay as they were, and the diff hunks are
built from the same edits. A server that has no `args` gets the key inserted.

**Backups and manifest hold no secrets.** Each file is copied first, with mode 0600 in a 0700
directory under `<data-dir>/install/backups`. The manifest (`<data-dir>/install/manifest.json`)
stores only paths, sha256 hashes and edit offsets, never config content. A test scans the diffs and
the manifest for the fake secrets.

**Atomic, and refuses to race the app.** Writes go to a temp file, are fsynced and renamed over the
original, and the file mode is kept. If the app changed the file after mcpgraph read it, the write is
refused. A symlinked config keeps its link; the real file is rewritten. `install` also refuses a
`go run` binary, or a binary not named `mcpgraph`, unless you pass `--bin`, because scans recognise
proxied servers by the basename.

**Restore or unwrap.** If a file is exactly as install left it, `uninstall` restores the backup byte
for byte. If the app changed it since (Claude Code rewrites `~/.claude.json` all the time), a restore
would lose those changes, so only the proxy wrapping is removed, in place. Then the backups are
deleted. Several installs are undone newest first.

**The diff is redacted.** Secrets inside JSON string literals are masked in the printed diff, for
example `"GITHUB_PERSONAL_ACCESS_TOKEN": "****1234"`,
`"postgresql://admin:****@db.internal:5432/prod"` and `"--api-key", "****FAKE"`.

**R4 clears.** On the fixtures, `report` shows 6 R4 findings before install, 0 after, and 6 again
after uninstall, and every file is byte-identical to the start
([`20`](docs/logs/20-install-dry-run.log), [`21`](docs/logs/21-install-apply.log),
[`22`](docs/logs/22-uninstall.log)).

## Coding-agent hook

A coding agent runs shell commands itself (`psql`, `rm`, `git push`). Those never touch MCP, so the
[proxy](#mcp-proxy) cannot see them. `mcpgraph hook` covers that path. Both interception points feed
the same policy engine, audit log and graph:

```mermaid
flowchart LR
  M["MCP tools/call<br/>AI app to MCP server"] --> PX["mcpgraph proxy"]
  SH["Bash, Read, Edit, Write<br/>coding agent, no MCP"] --> HK["mcpgraph hook"]
  PX --> PE["policy.Judge<br/>one rule set"]
  HK --> PE
  PE --> AL["audit.jsonl"]
  AL --> GR["graph<br/>Decision nodes"]
  AL -. "only if sink.json exists" .-> SK["collector"]
```

`mcpgraph hook claude-code|codex` is a `PreToolUse` hook. The agent starts it before each tool use
and writes one JSON payload to its stdin. mcpgraph judges it with the same rules as the proxy,
appends the decision to the audit log (`source: hook`) and answers in the agent's format.

```sh
mcpgraph hook claude-code --print-config   # prints the settings to add; writes nothing
mcpgraph hook codex --print-config
```

For Claude Code, merge the output into `~/.claude/settings.json` (the path below is the binary you
run it with):

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash|Read|Edit|Write|mcp__.*",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/mcpgraph",
            "args": ["hook", "claude-code"],
            "timeout": 10,
            "onFailure": "block"
          }
        ]
      }
    ]
  }
}
```

Codex uses `~/.codex/hooks.json` and the matcher `Bash|apply_patch|mcp__.*`. Only `mcpgraph install`
edits app configs, so `--print-config` just prints.

| Verdict | What the hook does |
| --- | --- |
| allow | **Prints nothing**, exit 0. The agent's own permission prompt still applies; mcpgraph never auto-approves. |
| deny | Prints a `hookSpecificOutput` deny answer. The reason names the rule ID, so the model can tell the user why. |
| ask | Claude Code: an `ask` answer, and the agent prompts. Codex has no `ask`, so the hook denies and explains how to approve. |
| any failure | Bad payload, bad policy, unknown agent or a non-`PreToolUse` event: reason on stderr, **exit 2**, which blocks the call. Fail closed (hard rule 4). |

| | Claude Code | Codex |
| --- | --- | --- |
| Config file | `~/.claude/settings.json` | `~/.codex/hooks.json` |
| Matcher | `Bash\|Read\|Edit\|Write\|mcp__.*` | `Bash\|apply_patch\|mcp__.*` |
| `ask` | native | not supported; deny with approval hint |
| If the hook crashes or times out | **fails open** by default, so the printed config sets `"onFailure": "block"` | the docs list no failure setting (`TODO(verify)`); mcpgraph itself exits 2 on every error it can detect |
| Command form | `command` + `args` (no shell) | one shell string, quoted by mcpgraph |
| Payload | captured from a real run ([`testdata/hooks/claude-code/captured-bash-echo.json`](testdata/hooks/claude-code/captured-bash-echo.json)) | written from the official docs and JSON schema in `openai/codex` |

**A real run.** In Claude Code 2.1.302, `claude -p` was asked to run `psql -c "DROP TABLE users"`.
The hook blocked it, and the model reported:

```text
PreToolUse:Bash hook error: mcpgraph blocked this by rule block-destructive-sql: Destructive SQL is blocked: ...
```

The audit log got a `deny` line with `source: hook` and `agent: claude-code`
([`docs/logs/23-hook-claude-code.log`](docs/logs/23-hook-claude-code.log)). The Codex payloads
([`24`](docs/logs/24-hook-codex.log)) and the resulting events ([`25`](docs/logs/25-hook-events.log))
are replayed the same way, so the docs do not need a live agent:

<p align="center">
  <img src="docs/images/23-hook.svg" alt="mcpgraph hook" width="100%">
</p>

**Honest status for Codex.** The format was verified against the official docs, but a real
`codex exec` run (0.162.1) did not invoke the project-local hook, so Codex end to end is still
`TODO(verify)`. Hooks also see only what the agent reports in the payload (see
[Limitations](#limitations)).

## Audit log and events

Every decision is appended as one JSON line to `audit.jsonl` in mcpgraph's data directory
(`~/.local/share/mcpgraph`, or `~/Library/Application Support/mcpgraph` on macOS; override with
`--data-dir`):

```json
{"time":"2026-10-18T09:01:00Z","source":"hook","agent":"claude-code","app":"Claude Code","tool":"Bash","summary":"PGPASSWORD=****dXyz psql -h db -c \"DROP TABLE users\"","rule_id":"block-destructive-sql","verdict":"deny","reason":"Destructive SQL is blocked: ..."}
```

- **Append-only.** The file is opened with `O_APPEND` and mode 0600, and each decision is a single
  `write`. Every hook call is a separate process, and several proxies can log at once; the kernel
  appends each line at the end of the file, so lines never overwrite each other. mcpgraph never
  rewrites or truncates the file.
- **Redacted twice.** The policy engine stores only the summary that `redact.Text` produced, and
  `audit.Log.Append` redacts the summary and the reason again before writing. Redaction is
  idempotent, so the second pass changes nothing that was already masked.
- **Robust reading.** A torn last line (after a crash) or a malformed line is skipped and counted,
  not fatal. Lines longer than `bufio.Scanner`'s 64 KiB default are accepted.

`mcpgraph events` lists and filters decisions (`--verdict`, `--source`, `--agent`, `--tool`,
`--rule`, `--since 24h|7d|RFC3339`, `-n`, `-f table|jsonl`):

<p align="center">
  <img src="docs/images/14-events.svg" alt="mcpgraph events" width="100%">
</p>

`scan`, `report` and `export` add the newest decisions (`--decisions`, default 50) to the graph as
`Decision` nodes. The graph builder links each one with `DECIDED_ON`: a proxy decision to its MCP
server, a hook decision to its AI app. Below are the decisions above on the fixture graph
([`docs/logs/17-export-decisions-mermaid.log`](docs/logs/17-export-decisions-mermaid.log)):

```mermaid
flowchart LR
  classDef AIApp fill:#dbeafe,stroke:#1d4ed8,color:#000000
  classDef MCPServer fill:#ede9fe,stroke:#6d28d9,color:#000000
  classDef Decision fill:#fce7f3,stroke:#be185d,color:#000000
  n_AIApp_e6e695d8b633["AIApp<br/>Claude Code"]:::AIApp
  n_Decision_5a40b3c6668c["Decision<br/>allow Bash"]:::Decision
  n_Decision_645a5a65bd29["Decision<br/>deny Bash (block-secret-read)"]:::Decision
  n_Decision_68869cc998bc["Decision<br/>deny read_file (block-secret-read)"]:::Decision
  n_Decision_83d6460361e4["Decision<br/>allow query"]:::Decision
  n_Decision_a19651063d3c["Decision<br/>ask Bash (ask-recursive-delete)"]:::Decision
  n_Decision_a27401665883["Decision<br/>deny Bash (block-destructive-sql)"]:::Decision
  n_Decision_c5f7ca59a919["Decision<br/>deny query (block-destructive-sql)"]:::Decision
  n_MCPServer_b3a12a332024["MCPServer<br/>Claude Desktop / filesystem"]:::MCPServer
  n_MCPServer_e88807ebc6c1["MCPServer<br/>Claude Desktop / postgres"]:::MCPServer
  n_Decision_5a40b3c6668c -->|DECIDED_ON| n_AIApp_e6e695d8b633
  n_Decision_645a5a65bd29 -->|DECIDED_ON| n_AIApp_e6e695d8b633
  n_Decision_68869cc998bc -->|DECIDED_ON| n_MCPServer_b3a12a332024
  n_Decision_83d6460361e4 -->|DECIDED_ON| n_MCPServer_e88807ebc6c1
  n_Decision_a19651063d3c -->|DECIDED_ON| n_AIApp_e6e695d8b633
  n_Decision_a27401665883 -->|DECIDED_ON| n_AIApp_e6e695d8b633
  n_Decision_c5f7ca59a919 -->|DECIDED_ON| n_MCPServer_e88807ebc6c1
```

The decisions above come from [`tools/replay`](tools/replay/main.go), which judges recorded actions
([`testdata/actions/demo.jsonl`](testdata/actions/demo.jsonl)) with the shipped policy and appends
them exactly as the proxy and the hook do, so the docs do not depend on a live agent.

## Central collector (opt-in)

By default nothing leaves the machine (hard rule 3). The event sink is **off unless
`<data-dir>/sink.json` exists**, and only `mcpgraph sink set` creates it. When it is on, the redacted
decisions in the audit log are shipped to a collector, so one place shows what every laptop's agents
tried to do.

```mermaid
flowchart LR
  subgraph A["laptop-a"]
    AA["proxy + hook"] --> LA["audit.jsonl"]
    LA -- "sink: offset file" --> SA["shipper"]
  end
  subgraph B["laptop-b"]
    BB["proxy + hook"] --> LB["audit.jsonl"]
    LB -- "sink: offset file" --> SB["shipper"]
  end
  SA -- "POST /v1/events" --> C["mcpgraph collector<br/>dedup by event id"]
  SB -- "POST /v1/events" --> C
  C --> EV["events.jsonl<br/>0600"]
  EV --> Q["GET /v1/events, /v1/hosts<br/>mcpgraph events --collector"]
```

```sh
# on the collector machine (loopback by default)
mcpgraph collector --listen 127.0.0.1:7600 --data ./collector-data
mcpgraph collector --listen 0.0.0.0:7600 --data ./collector-data --token SECRET   # accept other machines

# on each laptop
mcpgraph sink set https://collector.example:7600 --token SECRET --host laptop-a
mcpgraph sink status    # on/off, url, host, masked token, events not shipped yet
mcpgraph sink flush     # ship the pending events now
mcpgraph sink off       # removes sink.json

# fleet-wide view
mcpgraph events --collector https://collector.example:7600 --token SECRET --verdict deny
```

**Ships from the audit log.** The proxy and the hook never send anything themselves. They append to
`audit.jsonl` as before. A shipper reads the log from a saved offset (`sink.offset`), POSTs batches
and moves the offset only after a 2xx. Each event ID is the sha256 of its audit line, so delivery is
at-least-once and the collector drops repeats. Network trouble can delay shipping but never loses or
changes a decision ([decision note](wiki/decisions/2026-10-18-ship-from-audit-log.md)).

**Never slows the agent much.** An exclusive `flock` on `sink.lock` keeps concurrent proxies and
hook calls from sending the same lines twice. A request has a 2 s timeout in the proxy and a 300 ms
budget in a hook call, since the agent waits for that process. Whatever is not sent now goes out
with the next decision.

**Redacted again.** Summary and reason are re-redacted before they leave the machine, and the
collector redacts them once more. The token is never printed: `sink status` masks it. `sink set`
warns about plain `http://` to a non-loopback host.

**The collector.** `POST /v1/events`, `GET /v1/events`, `GET /v1/hosts` and `GET /healthz`. It listens
on loopback by default and warns when bound elsewhere without `--token`. Bodies over 4 MiB are
rejected, and `events.jsonl` is created with mode 0600. Dedup state is rebuilt from that file at
start, so a restart does not forget what it has seen.

**Acceptance.** Two agents (`laptop-a`, `laptop-b`) report into one collector: 0 duplicates and 0
secrets ([`docs/logs/26-collector.log`](docs/logs/26-collector.log)).

## Architecture

```mermaid
flowchart LR
  subgraph C["Collectors (one goroutine each)"]
    direction TB
    CFG["config collector<br/>4 app parsers"]
    PROC["process collector<br/>gopsutil"]
    H["host collector"]
  end
  CH(["chan model.Event"])
  subgraph B["Graph builder (single writer)"]
    direction TB
    AP["Apply(event)<br/>replace per Source"]
    G["Graph()<br/>dedupe · drop dangling · link RUNS_AS"]
    AP --> G
  end
  subgraph K["Consumers"]
    direction TB
    R["posture rules<br/>R1–R7"]
    REP["CLI report<br/>table · JSON · Mermaid"]
  end
  CFG --> CH
  PROC --> CH
  H --> CH
  CH --> AP
  G -- "immutable *Graph" --> R
  R --> REP
```

| Package | Role |
| --- | --- |
| `internal/model` | Node, Edge, Graph, Event, Finding types; stable node IDs |
| `internal/collector/config` | One parser per app, candidate paths, derivation of credentials and data sources |
| `internal/collector/process` | AI app processes, their children and MCP-like processes |
| `internal/graph` | Single-writer builder, RUNS_AS linking, snapshot diff |
| `internal/redact` | Secret detection and masking |
| `internal/posture` | Posture rules R1–R7, one file each |
| `internal/policy` | Runtime policy: YAML loading, SQL classifier, shell splitting, path globs, `Engine.Decide`, fail-closed `Judge` |
| `policies` | The shipped `default.yaml`, embedded with `//go:embed` |
| `internal/report` | Inventory and findings tables, JSON, Mermaid |
| `internal/hook` | Coding-agent adapters (Claude Code, Codex): payload to `policy.Action`, decision to the agent's answer, `--print-config` snippets |
| `internal/sink` | Opt-in shipping: `sink.json`, offset file, event IDs, `flock`, batches with short timeouts |
| `internal/central` | The collector: HTTP handlers, bearer auth, dedup store over `events.jsonl` |
| `internal/audit` | Append-only JSONL decision log, filters, and the collector that turns decisions into graph nodes |
| `internal/install` | `install` / `uninstall`: span-preserving JSON/JSONC edits, plan and diff, backups, manifest, atomic writes |
| `internal/proxy` | stdio MCP proxy: exact-key message parsing, `tools/call` judging, tool inventory, child process |
| `tools/termsvg` | Renders terminal output as the SVG captures in this README |
| `tools/replay` | Judges recorded actions and appends the decisions to an audit log (docs and tests) |
| `tools/fakemcp` | Scripted stdio MCP server for the docs; it runs nothing |

### Graph model

```mermaid
flowchart LR
  Host["Host<br/>hostname · os · arch · scanned_at"]
  App["AIApp<br/>name · config_path"]
  Srv["MCPServer<br/>name · command · args* · transport · url* · env keys · proxied"]
  Proc["Process<br/>pid · ppid · exe · cmdline*"]
  Cred["Credential<br/>kind (env|file|inline) · key_name · location<br/>never the value"]
  DS["DataSource<br/>kind (filesystem|database|saas) · identifier"]
  App -->|CONFIGURES| Srv
  Srv -->|RUNS_AS| Proc
  Srv -->|USES_SECRET| Cred
  Srv -->|ACCESSES| DS
  Dec["Decision<br/>time · source · agent · tool · summary* · rule_id · verdict"]
  Proc -->|CHILD_OF| Proc
  Dec -->|DECIDED_ON| Srv
  Dec -->|DECIDED_ON| App
```

<sub>* redacted before the node is created</sub>

### One scan, step by step

```mermaid
sequenceDiagram
  autonumber
  participant CLI as mcpgraph scan
  participant EG as errgroup
  participant Col as collectors (host, config, process)
  participant B as builder goroutine
  participant Ru as rules
  CLI->>EG: errgroup.WithContext(ctx)
  par one goroutine per collector
    EG->>Col: Collect(ctx, events)
    Col-->>B: model.Event{Source, Nodes, Edges, Warnings}
  end
  Note over Col,B: a broken config becomes a warning, not an error
  EG-->>CLI: g.Wait() returns
  CLI->>B: close(events)
  B->>B: Apply each event, then Graph(): dedupe, link RUNS_AS
  B-->>CLI: immutable *model.Graph
  CLI->>Ru: Evaluate(graph, R1..R7)
  Ru-->>CLI: findings sorted by severity
  CLI-->>CLI: print, exit 2 if any high
```

## Design decisions

**Single writer.** Only the graph builder goroutine changes graph state. Collectors never touch the
graph; they send `Event`s over a channel. So the graph needs no mutex, and there is no lock ordering
to get wrong. Each event carries a `Source` (for example `config:/home/u/.cursor/mcp.json`), and a
new event from that source replaces everything it sent before. This is how a re-scan of one config
file can remove a deleted server without rebuilding the rest of the graph. Readers only see
finished graphs: the builder publishes a new `*Graph` that shares no memory with its own state
(`Graph.Snapshot()` is a deep copy).

**Context everywhere.** There is one root `context.Context`, cancelled on Ctrl-C or SIGTERM.
Collectors run under `errgroup.WithContext`, so one failing component cancels the others. Every
channel send `select`s on `ctx.Done()`, so nothing blocks forever after shutdown. Partial failures
are not fatal: a broken config file or an unreadable process table becomes a warning on the event,
not an error.

**Collectors are interfaces.** `collector.Collector` has two methods (`Name`, `Collect`). Inside the
config collector, each app is a `parser` with `App()` and `Parse()`. To add an app, you add a parser
file, a line in `paths.go` and a fixture.

**Stable node IDs.** `ID = kind + ":" + sha256(stable key)[:12]`. The key is:

- config path + JSON path for servers,
- location for credentials,
- pid + start time for processes.

The same server has the same ID in every scan. That makes `graph.Diff` a comparison by ID, which is
what will let drift alerts say `new MCP server "notion" added to Cursor config`.

**Secrets are dropped at the edge.**

```mermaid
flowchart LR
  F["config file<br/>(raw JSON / JSONC)"] --> P["parser<br/>unexported serverConfig"]
  P --> D["derive.go<br/>only reader of raw values"]
  D -->|"args, URL, command"| RD["redact.Args / redact.URL<br/>****abcd"]
  D -->|"env values"| X["dropped<br/>(only key names kept)"]
  D -->|"secret detected"| CR["Credential node<br/>kind · key · location"]
  RD --> N["graph nodes"]
  CR --> N
  N --> O["table · JSON · Mermaid"]
  classDef safe fill:#dcfce7,stroke:#15803d,color:#000
  classDef raw fill:#fee2e2,stroke:#b91c1c,color:#000
  class F,P,D raw
  class RD,CR,N,O,X safe
```

The raw parsed config type is unexported, so it cannot leave the config package. Masking happens
once, at collection time, so every output is safe by construction. `internal/scan/leak_test.go`
runs the fixtures through every output format and fails if any fake secret appears. `make docs` checks the generated logs and images the same way.

**Read-only, almost no network.** Scanning never writes outside mcpgraph's own data directory and
makes no outbound connections. The only outbound path is the opt-in [event sink](#central-collector-opt-in),
which does nothing unless `<data-dir>/sink.json` exists. `mcpgraph install` / `uninstall` are the
only commands that edit AI-app configs, with backups, a printed diff and `--yes`; `hook
--print-config` only prints.

**No cgo.** gopsutil is pure Go. `CGO_ENABLED=0` builds static binaries
for darwin/linux × amd64/arm64.

## Testing and quality gates

```sh
make build    # CGO_ENABLED=0 go build ./cmd/...
make test     # go test -race ./... (without -race if no C compiler is installed)
make bench    # policy engine benchmarks
make lint     # golangci-lint run
make scan     # go run ./cmd/mcpgraph scan
make release  # goreleaser build --snapshot --clean
make docs     # regenerate docs/logs and docs/images
make demo     # the DROP TABLE demo: PostgreSQL in Docker, Claude Code + MCP proxy
```

Tests never read the real machine. They build a fake `$HOME` in `t.TempDir()` from
`testdata/configs`, and the process collector is tested through a fake `Lister`.

| What | Test |
| --- | --- |
| No secret in any output (table, JSON, Mermaid, findings, diff, audit log, events) | `internal/scan/leak_test.go`, `cmd/mcpgraph/main_test.go` |
| Every posture rule: positive and negative cases | `internal/posture/r*_test.go` |
| Concurrent producers, single writer (run with `-race`) | `internal/graph/builder_test.go` |
| A `mcpgraph proxy -- ...` entry is recognised as proxied | `internal/collector/config/proxied_test.go` |
| Every runtime rule: positive, negative and tricky cases | `internal/policy/default_test.go` |
| Fail closed on engine error, panic or timeout | `internal/policy/engine_test.go` |
| No secret in a Decision (command, URL, headers, nested args) | `internal/policy/engine_test.go` |
| Under 5 ms per decision | `internal/policy/bench_test.go` |
| Audit log: append-only, 0600, concurrent writers, torn lines, long lines, redaction | `internal/audit/audit_test.go` |
| Replayed decisions become Decision nodes with the right `DECIDED_ON` edges | `internal/scan/replay_test.go` |
| `events` filters, formats and bad flags | `cmd/mcpgraph/events_test.go` |
| Proxy: recorded transcripts (one real SDK capture) replay; allowed calls unchanged, denials carry the rule ID, other messages byte-identical | `internal/proxy/proxy_test.go` |
| Proxy: fail closed on engine error or timeout, malformed input, 2 MiB and CRLF messages, batches, server exit, no secret in denials or audit | `internal/proxy/edge_test.go` |
| `mcpgraph proxy` end to end, bad policy fails closed | `cmd/mcpgraph/proxy_test.go` |
| Install round trip on the fixtures: only `command` / `args` change, install twice, filters, symlinks, missing `args`, app changed the file (refused on apply, unwrap on uninstall), config removed after install | `internal/install/install_test.go` |
| Install safety: no secret in diffs or manifest, backups 0600 in a 0700 dir, atomic writes | `internal/install/install_test.go` |
| JSONC span parser: comments, errors, line redaction | `internal/install/jsonscan_test.go` |
| `mcpgraph install` CLI: dry run writes nothing, round trip, refuses a test binary without `--bin` | `cmd/mcpgraph/install_test.go` |
| Hook: 13 payload cases across Claude Code and Codex, answer formats, allow prints nothing, 8 fail-closed cases, timeout gives `mcpgraph-fail-closed`, no secret in answers or audit | `internal/hook/hook_test.go`, `cmd/mcpgraph/hook_test.go` |
| Hook ships to a collector (`TestHookShipsToCollector`) | `cmd/mcpgraph/hook_test.go` |
| Sink: off without `sink.json`, offset, dedup IDs, `flock`, timeouts, redaction | `internal/sink/sink_test.go`, `cmd/mcpgraph/sink_test.go` |
| Collector: two agents report into one (`TestTwoAgentsReportToOneCollector`), dedup, 401, 413, restart | `internal/central/central_test.go` |
| Exit code 2 on high findings | `cmd/mcpgraph/main_test.go` |

<p align="center">
  <img src="docs/images/08-go-test-summary.svg" alt="go test" width="60%">
  <img src="docs/images/09-lint.svg" alt="golangci-lint" width="35%">
</p>

GoReleaser builds the binary for darwin/linux × amd64/arm64 as a static, stripped executable. See
[`docs/logs/10-release-build.log`](docs/logs/10-release-build.log).

## Run logs

`make docs` (`scripts/docs.sh`) runs everything below for real and writes the output to
[`docs/logs/`](docs/logs). Fixture runs show the temporary home as `/home/dev` and the hostname as
`devbox`. The real-machine log is unedited.

| Log | What it shows |
| --- | --- |
| [00-environment](docs/logs/00-environment.log) | commit, Go version, OS |
| [01-scan-fixtures](docs/logs/01-scan-fixtures.log) | `mcpgraph scan` on the fixtures (exit 2) |
| [02-report-fixtures](docs/logs/02-report-fixtures.log) · [03 json](docs/logs/03-report-fixtures-json.log) | findings as a table and as JSON |
| [04-export-mermaid](docs/logs/04-export-fixtures-mermaid.log) · [05 json](docs/logs/05-export-fixtures-json.log) | full graph exports |
| [06-scan-real-machine](docs/logs/06-scan-real-machine.log) | the machine that ran `make docs`, with processes |
| [07-go-test](docs/logs/07-go-test.log) · [08 summary](docs/logs/08-go-test-summary.log) | full verbose test run |
| [09-lint](docs/logs/09-lint.log) | golangci-lint |
| [10-release-build](docs/logs/10-release-build.log) | GoReleaser cross build + `file` output |
| [11-bench](docs/logs/11-bench.log) | policy engine benchmark |
| [12-policy-tests](docs/logs/12-policy-tests.log) | every runtime policy table case, by name |
| [13-replay-decisions](docs/logs/13-replay-decisions.log) | recorded actions judged and appended to the audit log |
| [14-events](docs/logs/14-events.log) · [15 deny as JSONL](docs/logs/15-events-deny-jsonl.log) | `mcpgraph events` |
| [16-scan-with-decisions](docs/logs/16-scan-with-decisions.log) | scan with a *RECENT DECISIONS* section |
| [17-export-decisions-mermaid](docs/logs/17-export-decisions-mermaid.log) | Decision nodes and their `DECIDED_ON` targets |
| [18-proxy-session](docs/logs/18-proxy-session.log) · [19 events](docs/logs/19-proxy-events.log) | a captured SDK session through `mcpgraph proxy`, and the decisions it wrote |
| [20-install-dry-run](docs/logs/20-install-dry-run.log) · [21 apply](docs/logs/21-install-apply.log) · [22 uninstall](docs/logs/22-uninstall.log) | `mcpgraph install` dry run; `--yes` and a report with no R4; `uninstall` and the byte-identical check |
| [23-hook-claude-code](docs/logs/23-hook-claude-code.log) · [24 codex](docs/logs/24-hook-codex.log) | `mcpgraph hook` on recorded Claude Code and Codex payloads |
| [25-hook-events](docs/logs/25-hook-events.log) | the hook decisions in the audit log |
| [26-collector](docs/logs/26-collector.log) | two agents report into one collector: 0 duplicates, 0 secrets |

Development history and design notes are kept in the [`wiki/`](wiki/index.md) knowledge bundle.

## Roadmap

The spec is [`AGENTS.md`](AGENTS.md). Posture (milestones 1–4), the policy engine (5), the audit log (6), the
MCP proxy (7), the installer (8), the coding-agent hook (9) and the event sink with a central
collector (11) are done.

| # | Milestone | Status |
| --- | --- | --- |
| 1–4 | Skeleton, discovery, redaction, graph + posture | done |
| 5 | Policy engine: YAML policy, matchers, SQL classifier, `Engine.Decide` | done |
| 6 | Append-only JSONL audit log, `mcpgraph events`, `Decision` nodes | done |
| 7 | `mcpgraph proxy -- <server>`: stdio MCP proxy that judges `tools/call` | done |
| 8 | `mcpgraph install` / `uninstall`: route servers through the proxy, with backups | done |
| 9 | `mcpgraph hook claude-code\|codex`: PreToolUse hook for coding agents | done |
| 10 | `make demo`: Postgres + DB MCP server, both paths blocking `DROP TABLE`; animated SVG | done |
| 11 | Opt-in event sink + `mcpgraph collector` | done |
| 12 | *(stretch)* daemon with drift alerts | stretch |

The v1 daemon, SQLite snapshot store and localhost API are kept on the
[`archive/v1-daemon`](https://github.com/pilsu-choi/mcpgraph/tree/archive/v1-daemon) branch and come
back with milestone 12.

## Limitations

- **Config locations.** Only `~/.claude.json` (Claude Code on Linux) has been checked on a real
  machine. The other paths in `paths.go` are marked `TODO(verify)`.
- **Project-scoped configs** (`.mcp.json`, `.cursor/mcp.json`, `.vscode/mcp.json` inside
  repositories) are not scanned. Only user-level files are.
- **Servers that are not in a config file are invisible.** Examples are servers provided by Claude
  Code plugins or VS Code extensions. They show up only as R6 shadow processes while they run.
- **Heuristics.** Secret detection (prefixes, secret-named keys, entropy), data-source inference and
  process matching are heuristics, so they can miss things or produce false positives. A server
  started through a wrapper script may not get a `RUNS_AS` edge.
- **App versions** are not detected yet.
- **Policy matching is pattern-based.** The SQL classifier does not parse string literals, so a
  keyword inside a string can count. The shell splitter does not expand variables, `$(...)` or
  backticks. Unresolved `rm` targets are treated as outside the project, which errs towards asking.
- **`ask-unknown-tool` needs a tool inventory.** Scans do not list tools, so the rule fires only
  in the proxy, after the server has answered `tools/list`.
- **The proxy speaks stdio only**, and cannot prompt for `ask`: it blocks with an explanation. The
  `_meta` key names (`mcpgraph/rule_id`, `mcpgraph/verdict`) follow the spec's prefix rules but are
  not registered anywhere (`TODO(verify)`).
- **Apps must be restarted** after `install` or `uninstall`; they read their MCP config at start.
  `install` also inherits the `TODO(verify)` config paths above, so for apps other than Claude Code
  on Linux it edits only the files it can find at the guessed locations.
- **Codex end to end is unverified.** The hook format comes from the official docs and schema, but a
  real `codex exec` run (0.162.1) did not invoke the project-local hook (`TODO(verify)`). What Codex
  does when a hook crashes is also undocumented.
- **Hooks see only what the agent reports.** A command built inside a script, or a tool the matcher
  does not list, is judged on the text in the payload or not at all. The hook is a guard rail, not a
  sandbox.
- **The collector is small on purpose.** It keeps events in memory plus `events.jsonl`, runs on a
  single node and has no TLS of its own. Put it behind a TLS proxy, or use an `https://` URL that
  terminates elsewhere. Delivery is at-least-once, with dedup by event ID.

- **The demo needs Docker, Node and a logged-in Claude Code.** Each `make demo` makes one real
  `claude -p` call on your account. The agent's wording varies from run to run; the recorded
  animation is one real run.

## Future work

- Detect which tools each server exposes. This needs talking MCP to the server, so it is no longer read-only.
- Project-scoped configs, found through the project list in `~/.claude.json` and recent workspaces.
- More apps (Windsurf, Zed, JetBrains, Continue), and Windows.
- eBPF / Endpoint Security process and network tracing, to see what a server actually reads and
  where it connects, not just what it is configured to reach.
- Remote (HTTP) MCP servers behind the proxy; the proxy handles stdio only.
- Central policy distribution and a fleet management UI.
- Signed releases and SBOMs from GoReleaser.
