# AGENTS.md — mcpgraph

Instructions for coding agents (Claude Code, etc.) working in this repository.
Read this whole file before writing code. When this file and a request conflict, ask.

## 1. What we are building

`mcpgraph` is an endpoint agent, written in Go, that watches and controls what AI agents do on a developer laptop. Think of it as a tiny EDR for AI agents.

It does two jobs:

1. **Visibility (posture).** Find which AI apps and MCP servers are installed, and build a graph of the processes, credentials and data sources they can reach. Flag risky configuration.
2. **Runtime control.** Intercept the actions an agent is about to take (an MCP tool call, a shell command from a coding agent), check them against a policy, and **allow, deny, or ask** before they run. Record every decision in an audit log.

The demo scenario that defines success:

> A developer asks Claude Code to "clean up the database". The agent tries to run `DROP TABLE users`. mcpgraph matches the policy rule `block-destructive-sql`, the command never runs, the agent is told why, and the decision shows up in the audit log and the graph.

Context: this is a portfolio project for a backend/platform engineering role at an AI endpoint security company. Code quality, clear design decisions and tests matter more than feature count.

## 2. The owner is learning Go

- Write idiomatic, boring Go. Prefer the standard library. No clever generics, no reflection-heavy code, no premature abstraction.
- Add a short comment wherever a Go idiom is non-obvious to someone coming from Python (context cancellation, channel ownership, `errgroup`, `io.Pipe`, `bufio.Scanner` limits, embedding, `defer` ordering, interface satisfaction).
- When you finish a milestone, summarize in 3–5 bullets which Go concepts it used, so the owner can explain them in an interview.

## 3. Hard rules (never break these)

1. **Read-only by default.** Scanning never modifies, moves or deletes anything and never signals processes. The only commands that write outside mcpgraph's own data directory are `mcpgraph install` and `mcpgraph uninstall`, which edit AI-app configs to route MCP servers through the proxy. `install` must back up every file it touches, print a diff, and require `--yes`; `uninstall` must restore the backups byte for byte. Tests must cover the round trip.
2. **Never store or print secret values.** A credential is recorded only as its kind, key name and location. Tool-call arguments and shell commands are redacted before they reach the audit log, the graph or any output. Tests must assert this.
3. **No network egress by default.** No telemetry, no update checks. The local API listens on `127.0.0.1` only. The only outbound path is the optional event sink (milestone 12), which is off unless the user configures a collector URL.
4. **Fail closed on `tools/call`, pass through everything else.** If the policy engine errors or times out while judging a tool call, the call is denied with a clear message. Other MCP traffic (initialize, list tools, notifications) is forwarded untouched. The proxy must never hang the agent session.
5. **No cgo.** Must cross-compile to a static binary for `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`.
6. **Do not guess external formats.** Config file paths, the MCP wire format and the hook input/output format of each coding agent must be checked against official docs or the owner's machine before code depends on them. Mark anything unverified with `// TODO(verify)` and list it in the milestone summary.

## 4. Scope

In scope (MVP):
- macOS and Linux
- Discovery: MCP configs for Claude Desktop, Claude Code, Cursor, VS Code; running AI app and MCP server processes
- Posture rules and a graph export (JSON, Mermaid)
- Policy engine: YAML policy, decisions `allow` / `deny` / `ask`
- Runtime control path A: stdio MCP proxy
- Runtime control path B: pre-tool-use hook for coding agents (Claude Code first)
- Append-only audit log of every decision; decisions appear as nodes in the graph
- Demo: the `DROP TABLE` scenario above, scripted and reproducible

Stretch (only after MVP is done):
- Optional event sink that ships redacted audit events to a self-hosted collector, plus a minimal collector that stores them (shows the "central management" half of EDR)
- Daemon mode with fsnotify re-scans and config drift alerts

Out of scope (mention as future work in README only):
- eBPF or kernel-level interception, network traffic interception
- Windows
- Remote MCP servers over HTTP (proxy handles stdio only)
- Central policy distribution and fleet management UI

## 5. Architecture

```
                        ┌──────────────── policy.yaml ────────────────┐
                        ▼                                             ▼
 Coding agent ──hook──▶ mcpgraph hook ──▶ ┌──────────────┐ ◀── mcpgraph proxy ◀── AI app
 (Claude Code)  stdin    (one process     │ policy engine│     (stdio, wraps each   (MCP client)
                JSON      per tool call)  └──────┬───────┘      MCP server)            │
                                                 │ decision                             ▼
                                                 ▼                                 MCP server
                                         audit log (JSONL) ──▶ graph builder ◀── collectors
                                                                    │           (configs, processes)
                                                                    ▼
                                                     report · JSON/Mermaid export · optional sink
```

Design decisions (keep them; explain them in the README):
- **Two interception points, one policy engine.** MCP tool calls are caught by the proxy; shell commands a coding agent runs directly never touch MCP, so they are caught by the hook. Both call the same `policy.Engine`, so a rule is written once.
- **The proxy is transparent.** It speaks newline-delimited JSON-RPC on stdio, inspects only `tools/call` requests, and copies everything else byte for byte. One goroutine per direction; a map of in-flight request IDs lets a denied call be answered with a JSON-RPC error without involving the server.
- **`ask` in the proxy.** MCP has no universal way to prompt the user from a proxy, so in the MVP `ask` is returned as a denial whose message says the action needs approval and how to allow it. The hook path supports `ask` natively if the coding agent does (verify).
- **Single writer for the graph.** Collectors and the audit log send events over a channel to one goroutine that owns the graph. No mutex on the graph; readers get a snapshot copy.
- **Context everywhere.** One root `context.Context`; cancelling it stops every goroutine. Use `errgroup.WithContext`.
- **Stable node IDs.** `ID = kind + ":" + sha256(stable key)[:12]`, so the same server keeps its ID across scans and runs.
- **Policy decisions are fast.** Target under 5 ms per decision; compile regexes once at load time; add a benchmark.

## 6. Repository layout

```
mcpgraph/
├── cmd/mcpgraph/            # one binary: scan, report, export, proxy, hook, install, uninstall, events
├── internal/
│   ├── model/               # Node, Edge, Graph, Event, Finding, Decision types
│   ├── collector/
│   │   ├── config/          # one file per app + paths.go
│   │   └── process/
│   ├── graph/               # builder (single writer), snapshot
│   ├── posture/             # posture rules R1–R6, one file per rule
│   ├── policy/              # YAML loading, matchers, Engine.Decide
│   ├── proxy/               # stdio JSON-RPC proxy
│   ├── hook/                # adapters per coding agent (claudecode first)
│   ├── install/             # config rewrite, backup, restore
│   ├── audit/               # append-only JSONL log
│   ├── redact/              # secret detection + masking
│   ├── sink/                # Sink interface; noop + optional HTTP sink (stretch)
│   └── report/              # table, JSON, Mermaid
├── policies/default.yaml    # shipped example policy
├── demo/                    # scripted DROP TABLE demo
├── testdata/                # configs, JSON-RPC transcripts, hook payloads
├── Makefile
├── .golangci.yml
├── .goreleaser.yaml
└── README.md
```

## 7. Data model

Nodes:

| Kind | Key fields |
| --- | --- |
| Host | hostname, os, arch, scanned_at |
| AIApp | name, version (if known), config_path |
| MCPServer | name, command, args, transport, source_config, proxied (bool) |
| Process | pid, ppid, exe, cmdline (redacted), started_at |
| Credential | kind (env/file/inline), key_name, location — never the value |
| DataSource | kind (filesystem/database/saas), identifier |
| Decision | time, source (proxy/hook), agent, tool, action summary (redacted), rule_id, verdict (allow/deny/ask) |

Edges: `CONFIGURES` (AIApp→MCPServer), `RUNS_AS` (MCPServer→Process), `USES_SECRET` (MCPServer→Credential), `ACCESSES` (MCPServer→DataSource), `CHILD_OF` (Process→Process), `DECIDED_ON` (Decision→MCPServer or AIApp).

```go
// The one interface both interception paths depend on.
type Engine interface {
    Decide(ctx context.Context, a Action) (Decision, error)
}

// Action is what an agent is about to do, normalized from either path.
type Action struct {
    Source string            // "proxy" or "hook"
    Agent  string            // e.g. "claude-code", or the MCP server name
    Tool   string            // MCP tool name, or "Bash" etc. for hooks
    Args   map[string]any    // raw arguments; redact before logging
    Text   string            // the command or query text the matchers inspect
}
```

## 8. Rules

Posture rules (static, from the graph):

| ID | Detects | Severity |
| --- | --- | --- |
| R1 | Plaintext secret in an MCP server's `env` or `args` | high |
| R2 | Filesystem MCP server rooted at `$HOME` or `/` | high |
| R3 | Unpinned package runner (`npx -y pkg`, `uvx pkg` with no version) | medium |
| R4 | MCP server not routed through the proxy | medium |
| R5 | Credential files (`~/.aws/credentials`, `~/.ssh/id_*`, `.env`) inside a path an MCP server can access | high |
| R6 | Running MCP-like process not declared in any config | medium |

Runtime policy (`policies/default.yaml`, evaluated per action, first match wins):

| Rule ID | Matches | Verdict |
| --- | --- | --- |
| block-destructive-sql | `DROP TABLE`, `DROP DATABASE`, `TRUNCATE`, `DELETE` without `WHERE` | deny |
| ask-recursive-delete | `rm -rf` outside the current project directory | ask |
| block-secret-read | reading `~/.ssh/*`, `~/.aws/credentials`, `.env` files | deny |
| ask-git-force-push | `git push --force` | ask |
| ask-unknown-tool | any MCP tool not seen in the last scan | ask |

Matchers in the MVP are case-insensitive regexes plus a small SQL statement classifier (no full SQL parser). Each rule gets a table-driven test with positive and negative cases, including tricky ones (comments, mixed case, `DELETE ... WHERE 1=1`, commands chained with `&&`).

## 9. Tech stack

- Go: current stable release; set it in `go.mod`
- CLI: `github.com/spf13/cobra`
- YAML: `gopkg.in/yaml.v3`
- Processes: `github.com/shirou/gopsutil/v4`
- Concurrency: `golang.org/x/sync/errgroup`
- Logging: `log/slog`
- JSON-RPC: standard `encoding/json`; write the small framing layer yourself
- Lint: `golangci-lint`; release: GoReleaser

Ask before adding any other dependency.

## 10. Commands

```
make build      # go build ./cmd/...
make test       # go test -race ./...
make bench      # go test -bench=. ./internal/policy/...
make lint       # golangci-lint run
make demo       # runs demo/ end to end
make release    # goreleaser build --snapshot --clean
```

Before declaring any task done: `make lint test` must pass.

## 11. Milestones

Work in order. Finish and commit one before starting the next. Runtime control (7–10) matters more than polish; if time is short, cut 6's report formatting, never 7–10.

1. **Skeleton** — go.mod, layout, cobra CLI stubs, `model` types, Makefile, CI running lint + test. *Done when:* `make build lint test` passes on a clean clone.
2. **Discovery** — config parsers for the four apps with verified paths, process collector. *Done when:* `mcpgraph scan` lists AI apps, MCP servers and their running processes on the owner's machine; parser tests pass from fixtures alone.
3. **Redaction** — `internal/redact`. *Done when:* fixtures with fake secrets go through every output format and no raw value appears.
4. **Graph + posture** — single-writer builder, rules R1–R6, table report, JSON and Mermaid export. *Done when:* every rule has positive/negative tests and the Mermaid export renders on GitHub.
5. **Policy engine** — YAML loading, matchers, SQL classifier, `Engine.Decide`, default policy. *Done when:* table tests for every runtime rule pass and the benchmark shows under 5 ms per decision.
6. **Audit log** — append-only JSONL, redacted, with `mcpgraph events` to list and filter; decisions become `Decision` nodes. *Done when:* a test replays decisions and the graph shows `DECIDED_ON` edges.
7. **MCP proxy** — `mcpgraph proxy -- <server command>`. *Done when:* a recorded JSON-RPC transcript replays through the proxy with allowed calls forwarded unchanged and a denied `tools/call` answered with a JSON-RPC error that carries the rule ID; a test proves non-`tools/call` messages are byte-identical.
8. **Install / uninstall** — rewrite configs so each stdio MCP server runs through the proxy. *Done when:* install → uninstall restores every file byte for byte, and posture rule R4 clears after install.
9. **Coding-agent hook** — `mcpgraph hook claude-code` reads the tool-use payload on stdin and returns the decision in the agent's expected format (verify the format first). *Done when:* recorded hook payloads produce the right allow/deny/ask output, and a manual run in Claude Code blocks `psql -c "DROP TABLE users"`.
10. **Demo + README** — `demo/` sets up a throwaway Postgres (or SQLite) DB and a DB MCP server, then shows both paths blocking `DROP TABLE`. README in English: problem, architecture diagram, design decisions (section 5), demo GIF, limitations, future work. GoReleaser builds all four targets. *Done when:* `make demo` runs end to end on a clean machine.
11. *(stretch)* **Event sink + collector** — opt-in HTTP sink and a minimal collector that stores events from several machines. *Done when:* two local agents report into one collector.
12. *(stretch)* **Daemon** — fsnotify re-scan and config drift alerts.

## 12. Code conventions

- Errors: wrap with `fmt.Errorf("parse cursor config %s: %w", path, err)`. A broken config for one app must not fail the whole scan; record a warning and continue.
- No global state except the root logger.
- Exported identifiers get doc comments.
- Tests live next to code; fixtures in `testdata/`. Use `t.TempDir()` and a fake `$HOME` so tests never read or modify the real machine.
- Keep functions short; prefer explicit loops over clever helpers.
- Commit messages: `<milestone>: <what>` (e.g. `m7: forward non-tools/call messages unchanged`).

## 13. When unsure

Stop and ask the owner instead of guessing on: external formats (section 3, rule 6), new dependencies, anything touching the hard rules, or scope additions.

## 14. Wiki (record every piece of work)

`wiki/` is a knowledge bundle in Open Knowledge Format (OKF v0.1). After any change:

- Add a dated entry (UTC) to `wiki/log.md` linking to the pages you touched.
- Update or add the relevant page: `milestones/`, `decisions/`, `components/`, `rules/`,
  `runbooks/`, `lessons/` (bugs found and what they taught) or `issues/` (open gaps).
- Every page starts with YAML frontmatter: `type` (required), plus `title`, `description`,
  `tags`, `timestamp` (ISO 8601 UTC), and `resource` (GitHub link) for pages about code.
- One concept per file. Link pages with relative Markdown links. Every directory has an `index.md`.
- If the change affects outputs, run `make docs` so `docs/logs` and `docs/images` stay current.

## 15. Owner decisions (recorded 2026-10-18; these override earlier sections)

- **v1 daemon archived.** The v1 daemon, SQLite store, localhost API and fsnotify watcher live on the
  `archive/v1-daemon` branch. They come back only for milestone 12 (stretch). `fsnotify` and
  `modernc.org/sqlite` stay approved for that milestone.
- **Plain-HTTP rule kept as R7.** v1's R4 (remote MCP over plain `http://` to a non-loopback host,
  medium) is kept as posture rule **R7**; R4 now means "not routed through the proxy".
- **Demo database is PostgreSQL.**
- **`make docs`** regenerates `docs/logs` and `docs/images` (the v1 `make demo`); `make demo` is the
  DROP TABLE demo from milestone 10.
- **Proxy denials use `isError` results** (supersedes the "JSON-RPC error" wording in milestone 7).
  A denied or ask-ed `tools/call` is answered with a normal result, `isError: true`, a text item
  that names the rule and the reason, and the rule ID and verdict in `_meta`. The MCP spec says
  clients SHOULD show tool errors to the model but only MAY show JSON-RPC errors; the agent must
  see why it was blocked.
- **Deny rules before ask rules** in `policies/default.yaml` (first match wins), so for example
  `rm -rf ~/.ssh` is denied rather than asked.
