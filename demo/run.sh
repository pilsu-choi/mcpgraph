#!/usr/bin/env bash
# The whole DROP TABLE demo (make demo): build mcpgraph, run both paths, clean up.
# Set SKIP_HOOK=1 to run only the proxy path (no Claude Code needed).
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

WORK=$(mktemp -d)
cleanup() {
	rm -rf "$WORK"
	compose down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT

step "Building mcpgraph"
mkdir -p "$WORK/bin"
build_mcpgraph "$WORK/bin"
export MCPGRAPH=$WORK/bin/mcpgraph

step "Path A: MCP proxy (Claude Desktop style)"
DEMO_DATA=$WORK/data-proxy "$DEMO_DIR/proxy-demo.sh"

if [ "${SKIP_HOOK:-}" = 1 ]; then
	step "Path B skipped (SKIP_HOOK=1)"
else
	step "Path B: coding-agent hook (Claude Code)"
	DEMO_DATA=$WORK/data-hook "$DEMO_DIR/hook-demo.sh" ||
		echo "path B did not complete (see the message above)" >&2
fi
step "Tearing down the demo database"
