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
