#!/usr/bin/env bash
# Regenerates docs/logs/*.log and docs/images/*.svg from real runs.
#
#   make docs            # or: scripts/docs.sh
#
# Fixture runs use a temporary fake $HOME filled from testdata/configs (all
# secrets there are fake). In those logs the temporary path is shown as
# /home/dev and the hostname as "devbox" so the output is stable and
# readable. The "real machine" log is left untouched: it is what mcpgraph
# sees on the machine that ran this script.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
LOGS=$ROOT/docs/logs
IMAGES=$ROOT/docs/images
mkdir -p "$LOGS" "$IMAGES"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
BIN=$WORK/bin
DEMO_HOME=$WORK/home
mkdir -p "$BIN" "$DEMO_HOME"

cd "$ROOT"
CGO_ENABLED=0 go build -o "$BIN/" ./cmd/mcpgraph
go build -o "$BIN/termsvg" ./tools/termsvg
go build -o "$BIN/replay" ./tools/replay
go build -o "$BIN/fakemcp" ./tools/fakemcp
export PATH=$BIN:$PATH

# --- fake home --------------------------------------------------------------
install_fixture() { mkdir -p "$(dirname "$DEMO_HOME/$2")"; cp "testdata/configs/$1" "$DEMO_HOME/$2"; }
install_fixture claude_desktop/claude_desktop_config.json .config/Claude/claude_desktop_config.json
install_fixture claude_code/.claude.json .claude.json
install_fixture cursor/mcp.json .cursor/mcp.json
install_fixture vscode/mcp.json .config/Code/User/mcp.json
# Fake credential files so rule R5 has something to find. Only their
# existence matters; mcpgraph never opens them.
mkdir -p "$DEMO_HOME/.aws" "$DEMO_HOME/.ssh"
echo "# FAKE demo file" > "$DEMO_HOME/.aws/credentials"
echo "# FAKE demo file" > "$DEMO_HOME/.ssh/id_ed25519"

HOST=$(hostname)
# tidy makes fixture output stable: temp path -> /home/dev, hostname -> devbox.
tidy() { sed -e "s|$DEMO_HOME|/home/dev|g" -e "s|$WORK|/tmp/mcpgraph-demo|g" -e "s|\b$HOST\b|devbox|g"; }

# log FILE SHOWN-COMMAND CMD... runs CMD, writes "$ SHOWN-COMMAND", the
# output and the exit code to docs/logs/FILE.
log() {
	local file=$1 shown=$2 code=0
	shift 2
	{
		echo "\$ $shown"
		"$@" 2>&1 || code=$?
		echo "# exit code: $code"
	} | tidy > "$LOGS/$file"
	# The block above ran in a subshell (it is part of a pipeline), so its
	# exit code is only in the log file.
	echo "wrote docs/logs/$file ($(tail -n 1 "$LOGS/$file" | sed 's/^# //'))"
}

# svg LOGFILE TITLE [FILTER...] renders a log as a terminal capture.
svg() {
	local file=$1 title=$2 out=${1%.log}.svg
	termsvg -title "$title" -o "$IMAGES/$out" < "$LOGS/$file"
	echo "wrote docs/images/$out"
}

# --- environment ------------------------------------------------------------
# Build the text first: "> file" truncates the file before the commands run,
# which would make git describe report a dirty tree.
ENV_INFO="# generated: $(date -u +%Y-%m-%dT%H:%M:%SZ)
# commit:    $(git describe --always --dirty) ($(git log -1 --format=%s))
# go:        $(go version)
# os:        $(uname -srm)"
echo "$ENV_INFO" > "$LOGS/00-environment.log"
echo "wrote docs/logs/00-environment.log"

# --- one-shot CLI against fixtures -----------------------------------------
log 01-scan-fixtures.log "mcpgraph scan --home /home/dev --no-processes" \
	mcpgraph scan --home "$DEMO_HOME" --no-processes
log 02-report-fixtures.log "mcpgraph report --home /home/dev --no-processes" \
	mcpgraph report --home "$DEMO_HOME" --no-processes
log 03-report-fixtures-json.log "mcpgraph report -f json --home /home/dev --no-processes" \
	mcpgraph report -f json --home "$DEMO_HOME" --no-processes
log 04-export-fixtures-mermaid.log "mcpgraph export -f mermaid --home /home/dev --no-processes" \
	mcpgraph export -f mermaid --home "$DEMO_HOME" --no-processes
log 05-export-fixtures-json.log "mcpgraph export -f json --home /home/dev --no-processes" \
	mcpgraph export -f json --home "$DEMO_HOME" --no-processes
# Real machine: no tidy, this is exactly what mcpgraph reports here.
{
	echo "\$ mcpgraph scan   # real \$HOME, with process discovery"
	code=0
	mcpgraph scan 2>&1 || code=$?
	echo "# exit code: $code"
} > "$LOGS/06-scan-real-machine.log"
echo "wrote docs/logs/06-scan-real-machine.log"

# --- runtime decisions: audit log, events, Decision nodes ------------------
# tools/replay judges recorded actions with the shipped policy and appends
# the decisions to the audit log, as `mcpgraph proxy` / `hook` will.
DATA=$WORK/data
log 13-replay-decisions.log "replay -data-dir /home/dev/.local/share/mcpgraph < testdata/actions/demo.jsonl" \
	sh -c "replay -data-dir '$DATA' -home /home/dev < testdata/actions/demo.jsonl"
log 14-events.log "mcpgraph events" mcpgraph events --data-dir "$DATA"
log 15-events-deny-jsonl.log "mcpgraph events --verdict deny -f jsonl" mcpgraph events --data-dir "$DATA" --verdict deny -f jsonl
log 16-scan-with-decisions.log "mcpgraph scan --home /home/dev --no-processes   # with the audit log above" \
	mcpgraph scan --home "$DEMO_HOME" --no-processes --data-dir "$DATA"
# Only the decision part of the graph: Decision nodes, the edges they
# start and the nodes those edges point at (awk reads the export twice).
mcpgraph export -f mermaid --home "$DEMO_HOME" --no-processes --data-dir "$DATA" > "$WORK/full.mmd"
{
	echo "\$ mcpgraph export -f mermaid   # Decision nodes and what they were decided on"
	awk 'NR == FNR { if ($0 ~ /DECIDED_ON/) want[$NF] = 1; next }
		/^flowchart|classDef (Decision|MCPServer|AIApp)|:::Decision|DECIDED_ON/ { print; next }
		/\[/ { id = $1; sub(/\[.*/, "", id); if (id in want) print }' "$WORK/full.mmd" "$WORK/full.mmd"
} | tidy > "$LOGS/17-export-decisions-mermaid.log"
echo "wrote docs/logs/17-export-decisions-mermaid.log"

# --- MCP proxy: a captured client session through mcpgraph proxy ---------
# The client side of a real SDK session is fed line by line (a short pause
# lets each answer come back first) into `mcpgraph proxy` wrapping fakemcp.
PDATA=$WORK/proxy-data
{
	echo "\$ mcpgraph proxy --server postgres --app 'Claude Desktop' -- fakemcp < testdata/jsonrpc/captured-sdk-1.30.0.jsonl"
	echo "# --- client -> proxy"
	grep -v '^#' testdata/jsonrpc/captured-sdk-1.30.0.jsonl
	echo "# --- proxy -> client"
	grep -v '^#' testdata/jsonrpc/captured-sdk-1.30.0.jsonl | while IFS= read -r l; do printf '%s\n' "$l"; sleep 0.2; done |
		mcpgraph proxy --data-dir "$PDATA" --server postgres --app "Claude Desktop" -- fakemcp 2>&1
	echo "# exit code: $?"
} | tidy > "$LOGS/18-proxy-session.log"
echo "wrote docs/logs/18-proxy-session.log"
log 19-proxy-events.log "mcpgraph events   # decisions the proxy wrote" mcpgraph events --data-dir "$PDATA"

# --- quality gates ----------------------------------------------------------
log 07-go-test.log "go test -v ./..." go test -count=1 -v ./...
log 08-go-test-summary.log "go test ./..." go test -count=1 ./...
log 09-lint.log "golangci-lint run" golangci-lint run
log 11-bench.log "go test -run='^\$' -bench=. -benchmem ./internal/policy/..." \
	go test -run='^$' -bench=. -benchmem ./internal/policy/...
log 10-release-build.log "goreleaser build --snapshot --clean && file dist/*/*" \
	sh -c 'goreleaser build --snapshot --clean 2>&1 | grep -E "building|succeeded|error" ; file dist/*/mcpgraph'

# --- runtime policy: one decision per line --------------------------------
# Uses the shipped policy on the actions the README talks about.
{
	echo "\$ go test -run 'TestBlockDestructiveSQL|TestAskRecursiveDelete|TestBlockSecretRead|TestAskGitForcePush|TestAskUnknownTool' -v ./internal/policy/"
	go test -count=1 -run 'TestBlockDestructiveSQL|TestAskRecursiveDelete|TestBlockSecretRead|TestAskGitForcePush|TestAskUnknownTool' -v ./internal/policy/ 2>&1 |
		grep -E -- '^\s+--- (PASS|FAIL)|^ok|^FAIL' | sed -E 's/ \([0-9.]+s\)$//; s/^ +//'
} > "$LOGS/12-policy-tests.log"
echo "wrote docs/logs/12-policy-tests.log"

# --- captures ---------------------------------------------------------------
svg 01-scan-fixtures.log "mcpgraph scan"
svg 02-report-fixtures.log "mcpgraph report"
svg 08-go-test-summary.log "go test"
svg 09-lint.log "golangci-lint"
svg 11-bench.log "make bench"
svg 14-events.log "mcpgraph events"
# Proxy capture: one line per answer the client got.
{
	echo "\$ mcpgraph proxy --server postgres -- fakemcp   # answers the client received"
	sed -n '/^# --- proxy -> client/,$p' "$LOGS/18-proxy-session.log" | grep '^{' |
		sed -E 's/.*"id":([0-9]+).*"text":"([^"]{0,95}).*/id \1  \2/; s/.*"id":([0-9]+).*"protocolVersion":"([^"]*)".*/id \1  initialize ok (protocol \2)/; s/.*"id":([0-9]+).*"tools":\[.*/id \1  tools: query, read_file/'
	echo
	cat "$LOGS/19-proxy-events.log"
} > "$WORK/proxy-short.log"
termsvg -title "mcpgraph proxy" -o "$IMAGES/18-proxy.svg" < "$WORK/proxy-short.log"
echo "wrote docs/images/18-proxy.svg"

# --- hard rule 2: no fake secret may appear in any generated file ------------
SECRETS=$(sed -n 's/^\t"\(.*\)",$/\1/p' internal/testutil/fakehome.go)
leaks=0
for secret in $SECRETS; do
	if grep -rlF -- "$secret" "$LOGS" "$IMAGES"; then
		echo "LEAK: $secret" >&2
		leaks=1
	fi
done
[ "$leaks" -eq 0 ] && echo "leak check: no fake secret in docs/logs or docs/images"
exit "$leaks"
