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
IHOME=$WORK/install-home
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
# Backup directory names (timestamp + random suffix) and "backup from <time>" are made stable too.
tidy() {
	sed -E -e "s|$DEMO_HOME|/home/dev|g" -e "s|$IHOME|/home/dev|g" -e "s|$WORK|/tmp/mcpgraph-demo|g" -e "s|\b$HOST\b|devbox|g" \
		-e 's/[0-9]{8}T[0-9]{6}Z-[0-9]+/BACKUP-ID/g' -e 's/(backup from )[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z/\1TIMESTAMP/g'
}

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

# --- install / uninstall: wrap the stdio servers, then undo it ------------
# --yes rewrites config files, so this works on a copy of the fake home.
rm -rf "$IHOME"
cp -a "$DEMO_HOME" "$IHOME"
log 20-install-dry-run.log "mcpgraph install --bin /usr/local/bin/mcpgraph   # dry run, writes nothing" \
	mcpgraph --home "$IHOME" install --bin /usr/local/bin/mcpgraph
log 21-install-apply.log "mcpgraph install --bin /usr/local/bin/mcpgraph --yes   # then: report" \
	sh -c "mcpgraph --home '$IHOME' install --bin /usr/local/bin/mcpgraph --yes
		echo; echo '\$ mcpgraph report --no-processes | grep -c R4   # rule R4, after install'
		n=\$(mcpgraph --home '$IHOME' report --no-processes | grep -c R4 || true); echo \"R4 findings: \$n\"
		echo; echo '\$ mcpgraph report --no-processes | grep R4   # for comparison: before install'
		mcpgraph --home '$DEMO_HOME' report --no-processes | grep R4 || true"
log 22-uninstall.log "mcpgraph uninstall --yes   # then compare with the untouched files" \
	sh -c "mcpgraph --home '$IHOME' uninstall --yes
		echo
		for f in .config/Claude/claude_desktop_config.json .claude.json .cursor/mcp.json .config/Code/User/mcp.json; do
			if cmp -s '$IHOME'/\$f '$DEMO_HOME'/\$f; then echo \"byte-identical  /home/dev/\$f\"; else echo \"DIFFERENT  /home/dev/\$f\"; fi
		done"

# --- hooks: Claude Code and Codex PreToolUse payloads ----------------------
# Payloads come from testdata/hooks. The hook reads one JSON payload on
# stdin and answers on stdout; exit code 2 means it failed closed.
HDATA=$WORK/hook-data
payload_summary() { # FILE -> one line with tool_name and command / file_path / path
	python3 - "$1" <<'PY'
import json, sys
try:
    d = json.load(open(sys.argv[1]))
except Exception:
    print("payload: not valid JSON (truncated on purpose)")
    sys.exit()
ti = d.get("tool_input") or {}
if "secret" in sys.argv[1]:
    print("tool_name: %s" % d.get("tool_name"))
    sys.exit()
what = ti.get("command") or ti.get("file_path") or ti.get("path") or json.dumps(ti)
print("tool_name: %s   input: %s" % (d.get("tool_name"), str(what).replace("\n", " | ")))
PY
}
hook_demo() { # AGENT FILE...: show each payload and what the hook answers
	local agent=$1 f code first=1
	shift
	for f in "$@"; do
		[ "$first" = 1 ] || echo
		first=0
		echo "\$ mcpgraph hook $agent < testdata/hooks/$agent/$f"
		payload_summary "testdata/hooks/$agent/$f"
		code=0
		mcpgraph hook "$agent" --home /home/testuser --data-dir "$HDATA" < "testdata/hooks/$agent/$f" > "$WORK/hook.out" 2> "$WORK/hook.err" || code=$?
		echo "stdout:"; sed 's/^/  /' "$WORK/hook.out"
		echo "stderr:"; sed 's/^/  /' "$WORK/hook.err"
		echo "exit code: $code"
	done
}
hook_claude() {
	hook_demo claude-code bash-drop-table.json bash-ls.json bash-rm-rf-home.json read-ssh-key.json mcp-delete.json captured-bash-echo.json malformed.json bash-secret-drop-table.json
	echo
	echo "\$ mcpgraph hook claude-code --print-config --bin /usr/local/bin/mcpgraph"
	mcpgraph hook claude-code --print-config --bin /usr/local/bin/mcpgraph
}
hook_codex() {
	hook_demo codex bash-drop-table.json bash-ls.json bash-rm-rf-home.json apply-patch-env.json mcp-delete.json malformed.json
	echo
	echo "\$ mcpgraph hook codex --print-config --bin /usr/local/bin/mcpgraph"
	mcpgraph hook codex --print-config --bin /usr/local/bin/mcpgraph
}
log 23-hook-claude-code.log "mcpgraph hook claude-code < testdata/hooks/claude-code/*.json   # one payload at a time" hook_claude
log 24-hook-codex.log "mcpgraph hook codex < testdata/hooks/codex/*.json   # ask becomes deny with an explanation" hook_codex
log 25-hook-events.log "mcpgraph events   # decisions the hooks wrote" mcpgraph events --data-dir "$HDATA"

# --- collector: two laptops ship decisions to one central server -----------
CPORT=$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1])')
CURL=http://127.0.0.1:$CPORT
SINK_A=$WORK/laptop-a
SINK_B=$WORK/laptop-b
mkdir -p "$SINK_A" "$SINK_B"
cp "$HDATA/audit.jsonl" "$SINK_A/audit.jsonl"
mcpgraph collector --listen "127.0.0.1:$CPORT" --data "$WORK/central" > "$WORK/collector.log" 2>&1 &
COLLECTOR_PID=$!
trap 'kill "$COLLECTOR_PID" 2>/dev/null || true; rm -rf "$WORK"' EXIT
for _ in $(seq 1 50); do curl -sf "$CURL/healthz" > /dev/null 2>&1 && break; sleep 0.1; done
collector_demo() {
	echo "# started in the background: mcpgraph collector --listen 127.0.0.1:7600 --data /srv/mcpgraph"
	echo "\$ curl -s http://127.0.0.1:7600/healthz"
	curl -s "$CURL/healthz"; echo
	echo
	echo "# laptop A: switch the sink on, then ship the decisions it already has"
	echo "\$ mcpgraph sink set http://127.0.0.1:7600 --host laptop-a"
	mcpgraph sink set "$CURL" --host laptop-a --data-dir "$SINK_A" 2>&1
	echo "\$ mcpgraph sink status"
	mcpgraph sink status --data-dir "$SINK_A" 2>&1
	echo "\$ mcpgraph sink flush"
	mcpgraph sink flush --data-dir "$SINK_A" 2>&1
	echo
	echo "# laptop B: every hook decision is shipped as it happens"
	echo "\$ mcpgraph sink set http://127.0.0.1:7600 --host laptop-b"
	mcpgraph sink set "$CURL" --host laptop-b --data-dir "$SINK_B" 2>&1
	echo "\$ mcpgraph hook claude-code < testdata/hooks/claude-code/bash-drop-table.json"
	mcpgraph hook claude-code --home /home/testuser --data-dir "$SINK_B" < testdata/hooks/claude-code/bash-drop-table.json > /dev/null 2>&1 || true
	echo "\$ mcpgraph hook claude-code < testdata/hooks/claude-code/bash-ls.json"
	mcpgraph hook claude-code --home /home/testuser --data-dir "$SINK_B" < testdata/hooks/claude-code/bash-ls.json > /dev/null 2>&1 || true
	echo "\$ mcpgraph sink status"
	mcpgraph sink status --data-dir "$SINK_B" 2>&1
	echo
	echo "# the central view"
	echo "\$ curl -s http://127.0.0.1:7600/v1/hosts | python3 -m json.tool"
	curl -s "$CURL/v1/hosts" | python3 -m json.tool
	echo
	echo "\$ mcpgraph events --collector http://127.0.0.1:7600"
	mcpgraph events --collector "$CURL" 2>&1
	echo
	echo "\$ mcpgraph events --collector http://127.0.0.1:7600 --host laptop-b"
	mcpgraph events --collector "$CURL" --host laptop-b 2>&1
}
log 26-collector.log "mcpgraph collector + two laptops with mcpgraph sink" collector_demo
sed -i "s/$CPORT/7600/g" "$LOGS/26-collector.log"
kill "$COLLECTOR_PID" 2>/dev/null || true
wait "$COLLECTOR_PID" 2>/dev/null || true

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
# Install capture: the Claude Desktop part of the dry run.
{
	sed -n '1,6p' "$LOGS/20-install-dry-run.log"
	echo "..."
	sed -n '/^--- .*claude_desktop_config/,/^$/p' "$LOGS/20-install-dry-run.log" | head -n 13
	echo "..."
	grep '^Dry run' "$LOGS/20-install-dry-run.log"
} > "$WORK/install-short.log"
termsvg -title "mcpgraph install" -o "$IMAGES/20-install.svg" < "$WORK/install-short.log"
echo "wrote docs/images/20-install.svg"

# Hook capture: the DROP TABLE deny, the ask case and the events table.
{
	sed -n '1,/^exit code/p' "$LOGS/23-hook-claude-code.log"
	echo
	sed -n '/rm-rf-home.json$/,/^exit code/p' "$LOGS/23-hook-claude-code.log"
	echo
	cat "$LOGS/25-hook-events.log"
} > "$WORK/hook-short.log"
termsvg -title "mcpgraph hook" -o "$IMAGES/23-hook.svg" < "$WORK/hook-short.log"
echo "wrote docs/images/23-hook.svg"

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
