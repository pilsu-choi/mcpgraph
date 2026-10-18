#!/usr/bin/env bash
# Regenerates docs/logs/*.log and docs/images/*.svg from real runs.
#
#   make demo            # or: scripts/demo.sh
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
PORT=${MCPGRAPH_DEMO_PORT:-17878}
mkdir -p "$LOGS" "$IMAGES"

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
BIN=$WORK/bin
DEMO_HOME=$WORK/home
mkdir -p "$BIN" "$DEMO_HOME"

cd "$ROOT"
CGO_ENABLED=0 go build -o "$BIN/" ./cmd/mcpgraph ./cmd/mcpgraphd
go build -o "$BIN/termsvg" ./tools/termsvg
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
{
	echo "# generated: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
	echo "# commit:    $(git describe --always --dirty) ($(git log -1 --format=%s))"
	echo "# go:        $(go version)"
	echo "# os:        $(uname -srm)"
} > "$LOGS/00-environment.log"
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

# --- daemon + API scenario --------------------------------------------------
DATA=$WORK/data
mcpgraphd --home "$DEMO_HOME" --data-dir "$DATA" --no-processes --addr "127.0.0.1:$PORT" 2> "$WORK/daemon.raw" &
DAEMON=$!
sleep 1

API=$WORK/api.raw
api() { # api SHOWN CURL-ARGS...
	local shown=$1
	shift
	{ echo "\$ $shown"; curl -sS "$@"; echo; } >> "$API"
}
api "curl -s 127.0.0.1:$PORT/findings | head -c 600" "127.0.0.1:$PORT/findings" -o "$WORK/f.json"
head -c 600 "$WORK/f.json" >> "$API"; printf '\n...\n' >> "$API"

{ echo "\$ # edit Cursor config: add a \"notion\" server"; } >> "$API"
cat > "$DEMO_HOME/.cursor/mcp.json" <<'EOF'
{
  "mcpServers": {
    "remote-insecure": { "url": "http://10.0.0.5:8080/mcp?api_key=FAKEqueryKey1234567890abcd" },
    "local-http": { "url": "http://localhost:3000/mcp" },
    "git": { "command": "uvx", "args": ["mcp-server-git", "--repository", "~/code/project"], "env": { "GIT_TOKEN": "${GIT_TOKEN}" } },
    "notion": { "command": "npx", "args": ["-y", "@notionhq/notion-mcp-server"] }
  }
}
EOF
sleep 1.5
{ echo "\$ # edit Cursor config: remove \"remote-insecure\""; } >> "$API"
cat > "$DEMO_HOME/.cursor/mcp.json" <<'EOF'
{
  "mcpServers": {
    "local-http": { "url": "http://localhost:3000/mcp" },
    "git": { "command": "uvx", "args": ["mcp-server-git", "--repository", "~/code/project"], "env": { "GIT_TOKEN": "${GIT_TOKEN}" } },
    "notion": { "command": "npx", "args": ["-y", "@notionhq/notion-mcp-server"] }
  }
}
EOF
sleep 1.5

api "curl -s '127.0.0.1:$PORT/snapshots?limit=3'" "127.0.0.1:$PORT/snapshots?limit=3" -o "$WORK/s.json"
# Show snapshot metadata and change messages only, one per line.
grep -E '"(id|taken_at|node_count|high|medium|message)"' "$WORK/s.json" >> "$API"
api "curl -s -o /dev/null -w '%{http_code}' -X POST 127.0.0.1:$PORT/graph   # read-only" \
	-o /dev/null -w '%{http_code}' -X POST "127.0.0.1:$PORT/graph"
api "curl -s -o /dev/null -w '%{http_code}' -H 'Host: evil.example' 127.0.0.1:$PORT/graph   # DNS rebinding" \
	-o /dev/null -w '%{http_code}' -H 'Host: evil.example' "127.0.0.1:$PORT/graph"

kill -INT "$DAEMON"
code=0
wait "$DAEMON" || code=$?
{
	echo "\$ mcpgraphd --home /home/dev --no-processes --addr 127.0.0.1:$PORT"
	cat "$WORK/daemon.raw"
	echo "# (SIGINT sent) exit code: $code"
} | tidy > "$LOGS/07-daemon.log"
echo "wrote docs/logs/07-daemon.log"
tidy < "$API" > "$LOGS/08-api.log"
echo "wrote docs/logs/08-api.log"

{
	echo "\$ mcpgraphd --addr 0.0.0.0:7878   # must refuse"
	code=0
	mcpgraphd --addr 0.0.0.0:7878 --data-dir "$WORK/refused" 2>&1 || code=$?
	echo "# exit code: $code"
} > "$LOGS/09-api-refuses-non-loopback.log"
echo "wrote docs/logs/09-api-refuses-non-loopback.log"

# --- quality gates ----------------------------------------------------------
log 10-go-test.log "go test -v ./..." go test -count=1 -v ./...
log 11-go-test-summary.log "go test ./..." go test -count=1 ./...
log 12-lint.log "golangci-lint run" golangci-lint run
log 13-release-build.log "goreleaser build --snapshot --clean && file dist/*/*" \
	sh -c 'goreleaser build --snapshot --clean 2>&1 | grep -E "building|succeeded|error" ; file dist/*/mcpgraph dist/*/mcpgraphd'

# --- captures ---------------------------------------------------------------
svg 01-scan-fixtures.log "mcpgraph scan"
svg 02-report-fixtures.log "mcpgraph report"
# Daemon capture: keep "LEVEL message" for server, snapshot and API events
# so the picture stays readable; the full JSON log is in 07-daemon.log.
{
	head -n 1 "$LOGS/07-daemon.log"
	sed -nE 's/.*"level":"([A-Z]+)","msg":"(([^"\\]|\\.)*)"(.*)/\1  \2  \4/p' "$LOGS/07-daemon.log" |
		grep -E 'MCP server|snapshot saved|API listening|starting|AI app' |
		sed -E -e 's/\\"/"/g' -e 's/,"node_id":"[^"]*"\}$//' -e 's/,?"change":"[a-z]+","kind":"[A-Za-z]+"//' -e 's/\}$//' -e 's/  ,/  /'
	tail -n 1 "$LOGS/07-daemon.log"
} > "$WORK/daemon-short.log"
termsvg -title "mcpgraphd" -o "$IMAGES/07-daemon.svg" < "$WORK/daemon-short.log"
echo "wrote docs/images/07-daemon.svg"
# API capture: collapse the 33 "added" messages of the first snapshot.
awk '
	/"id": 1,/ { first = 1 }
	first && /"message"/ { n++; next }
	first && /^\$/ { printf "        ... %d changes (everything is new in the first snapshot)\n", n; first = 0 }
	{ print }
' "$LOGS/08-api.log" > "$WORK/api-short.log"
termsvg -title "localhost API" -o "$IMAGES/08-api.svg" < "$WORK/api-short.log"
echo "wrote docs/images/08-api.svg"
svg 11-go-test-summary.log "go test"
{ cat "$LOGS/09-api-refuses-non-loopback.log"; echo; cat "$LOGS/12-lint.log"; } > "$WORK/gates.log"
termsvg -title "safety checks" -o "$IMAGES/09-gates.svg" < "$WORK/gates.log"
echo "wrote docs/images/09-gates.svg"

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
