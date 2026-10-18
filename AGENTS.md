# AGENTS.md — mcpgraph

Instructions for coding agents (Claude Code, etc.) working in this repository.
Read this whole file before writing code. When this file and a request conflict, ask.

## 1. What we are building

`mcpgraph` is a single-binary, read-only Go agent that answers one question for a developer laptop:

> Which AI apps are installed, which MCP servers do they use, and which processes, credentials and data sources can those servers reach?

It discovers AI app configs and running processes, builds a graph of nodes and edges, evaluates risk rules against it, and outputs a CLI report, a JSON/Mermaid graph export, and a localhost-only API.

Context: this is a portfolio project for a backend/platform engineering role at an AI endpoint security company. Code quality, clear design decisions and tests matter more than feature count.

## 2. The owner is learning Go

- Write idiomatic, boring Go. Prefer the standard library. No clever generics, no reflection-heavy code, no premature abstraction.
- Add a short comment wherever a Go idiom is non-obvious to someone coming from Python (context cancellation, channel ownership, `errgroup`, embedding, `defer` ordering, interface satisfaction).
- When you finish a milestone, summarize in 3–5 bullets which Go concepts it used, so the owner can explain them in an interview.

## 3. Hard rules (never break these)

1. **Read-only.** The agent never modifies, moves or deletes files outside its own data directory, and never kills or signals processes.
2. **Never store or print secret values.** A credential is recorded only as its kind, key name and location (file path + JSON path or env var name). In any output, secret-looking values are masked as `****` plus at most the last 4 characters. Tests must assert this.
3. **No network egress.** The only listener is `127.0.0.1`. No telemetry, no update checks, no outbound calls.
4. **No cgo.** Must cross-compile to a static binary for `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`.
5. **Do not guess config paths.** Paths in `internal/collector/config/paths.go` are candidates. Verify each one on the actual machine (or ask the owner) before marking it confirmed. Keep unverified paths behind a `// TODO(verify)` comment.

## 4. Scope

In scope (MVP):
- macOS and Linux
- MCP config discovery and parsing for: Claude Desktop, Claude Code, Cursor, VS Code (add more only after these four work)
- Process discovery: running AI app processes and their child MCP server processes
- Graph building, rule evaluation, CLI report, JSON + Mermaid export
- Daemon mode: watch config files with fsnotify, re-scan, store snapshots in SQLite, report diffs
- Localhost read-only HTTP API

Out of scope (do not build; mention as future work in README only):
- eBPF, kernel hooks, network interception
- Windows
- Remote/central server (define a Go interface `Sink` for it, implement only a no-op)

## 5. Architecture

```
Collectors (each its own goroutine)          Graph builder            Consumers
┌───────────────────┐                     ┌──────────────────┐     ┌──────────────┐
│ config collector  │──┐                  │ single writer:   │────▶│ rule engine  │──┐
│ process collector │──┼── chan Event ───▶│ owns the Graph,  │     └──────────────┘  ├─▶ CLI report
│ config watcher    │──┘                  │ dedupes nodes    │────▶┌──────────────┐  ├─▶ JSON / Mermaid
└───────────────────┘                     └──────────────────┘     │ SQLite store │──┘─▶ localhost API
                                                                   └──────────────┘
```

Design decisions (keep them; explain them in the README):
- **Single writer.** Only the graph builder goroutine mutates the graph. Collectors send `Event`s on a channel. No mutex on the graph itself; readers get an immutable snapshot (`Graph.Snapshot()` returns a deep copy).
- **Context everywhere.** One root `context.Context`; cancelling it stops every collector. Use `errgroup.WithContext`.
- **Collectors are interfaces.** Adding an app = adding one parser file + testdata, nothing else.
- **Stable node IDs.** `ID = kind + ":" + sha256(stable key)[:12]`, so the same server across scans keeps the same ID and snapshots can be diffed.

## 6. Repository layout

```
mcpgraph/
├── cmd/
│   ├── mcpgraph/        # CLI: scan, report, export
│   └── mcpgraphd/       # daemon: watch + API
├── internal/
│   ├── model/           # Node, Edge, Graph, Event, Finding types
│   ├── collector/
│   │   ├── config/      # one file per app + paths.go
│   │   ├── process/
│   │   └── watch/
│   ├── graph/           # builder (single writer), snapshot, diff
│   ├── rules/           # one file per rule
│   ├── redact/          # secret detection + masking
│   ├── store/           # SQLite snapshots
│   ├── api/             # localhost HTTP
│   └── report/          # table, JSON, Mermaid
├── testdata/configs/    # sample configs per app, incl. ones with fake secrets
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
| MCPServer | name, command, args, transport (stdio/http), url, source_config |
| Process | pid, ppid, exe, cmdline (redacted), started_at |
| Credential | kind (env/file/inline), key_name, location — never the value |
| DataSource | kind (filesystem/database/saas), identifier (path, host, service) |

Edges: `CONFIGURES` (AIApp→MCPServer), `RUNS_AS` (MCPServer→Process), `USES_SECRET` (MCPServer→Credential), `ACCESSES` (MCPServer→DataSource), `CHILD_OF` (Process→Process).

```go
type Rule interface {
    ID() string
    Eval(g *model.Graph) []model.Finding
}
```

## 8. Rules (MVP)

| ID | Detects | Severity |
| --- | --- | --- |
| R1 | Plaintext secret in an MCP server's `env` or `args` (known token prefixes, high-entropy strings) | high |
| R2 | Filesystem MCP server rooted at `$HOME` or `/` | high |
| R3 | Unpinned package runner (`npx -y pkg` with no version, `uvx pkg` with no version) | medium |
| R4 | Remote MCP over plain `http://` to a non-loopback host | medium |
| R5 | Credential files (`~/.aws/credentials`, `~/.ssh/id_*`, `.env`) inside a path an MCP server can access | high |
| R6 | Running MCP-like process not declared in any config (shadow server) | medium |

Each rule: its own file, a table-driven test with at least one positive and one negative case, and a one-line rationale in the finding message.

## 9. Tech stack

- Go: current stable release; set it in `go.mod`
- CLI: `github.com/spf13/cobra`
- Processes: `github.com/shirou/gopsutil/v4`
- File watching: `github.com/fsnotify/fsnotify`
- SQLite: `modernc.org/sqlite` (pure Go)
- Concurrency: `golang.org/x/sync/errgroup`
- Logging: `log/slog` (JSON handler in daemon, text in CLI)
- HTTP: `net/http` standard library, Go 1.22+ routing patterns
- Lint: `golangci-lint`; release: GoReleaser

Ask before adding any other dependency.

## 10. Commands

Create a Makefile with these targets and keep them working:

```
make build      # go build ./cmd/...
make test       # go test -race ./...
make lint       # golangci-lint run
make scan       # go run ./cmd/mcpgraph scan
make release    # goreleaser build --snapshot --clean
```

Before declaring any task done: `make lint test` must pass.

## 11. Milestones

Work in this order. Finish and commit one before starting the next. Each has an acceptance check.

1. **Skeleton** — go.mod, layout, cobra CLI with `scan` stub, `model` types, Makefile, CI (GitHub Actions running lint + test). *Done when:* `make build lint test` passes on a clean clone.
2. **Config collector** — parsers for the four apps, `paths.go` with verified/unverified paths, testdata fixtures. *Done when:* `mcpgraph scan` lists AI apps and MCP servers from the owner's machine; parser tests pass from fixtures alone.
3. **Redaction** — `internal/redact` with masking and secret detection. *Done when:* a test feeds fixtures with fake secrets through the whole pipeline and asserts no raw value appears in any output format.
4. **Process collector** — match running processes to declared MCP servers; build `CHILD_OF` edges. *Done when:* a running MCP server shows `RUNS_AS` with its pid.
5. **Graph builder + exports** — single-writer builder, snapshot, JSON and Mermaid export. *Done when:* `mcpgraph export --format mermaid` renders correctly on GitHub.
6. **Rules R1–R6 + report** — table report grouped by severity; exit code 2 if any high finding (usable in CI). *Done when:* every rule has passing positive/negative tests.
7. **Daemon** — fsnotify watch, debounced re-scan, SQLite snapshots, diff events ("new MCP server added to Cursor config"). *Done when:* editing a fixture config while the daemon runs logs a diff within 2 seconds.
8. **Local API** — `GET /graph`, `GET /findings`, `GET /snapshots` on 127.0.0.1 only. *Done when:* a test asserts the server refuses to bind a non-loopback address.
9. **Release + README** — GoReleaser cross builds; README in English with the problem, architecture diagram, design decisions (section 5), sample output, limitations and future work. *Done when:* binaries build for all four targets.

## 12. Code conventions

- Errors: wrap with `fmt.Errorf("parse cursor config %s: %w", path, err)`. A broken config for one app must not fail the whole scan; record it as a warning and continue.
- No global state except the root logger.
- Exported identifiers get doc comments.
- Tests live next to code; fixtures in `testdata/`. Use `t.TempDir()` and fake `$HOME` so tests never read the real machine.
- Keep functions short; prefer explicit loops over clever helpers.
- Commit messages: `<milestone>: <what>` (e.g. `m2: add cursor config parser`).

## 13. When unsure

Stop and ask the owner instead of guessing on: config file locations, new dependencies, anything touching rule 3 (hard rules), or scope additions.
