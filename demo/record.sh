#!/usr/bin/env bash
# Records the README animation: docs/images/demo-draft.svg (default) or
# docs/images/demo.svg (--final).
#
#   demo/record.sh                         # draft: real proxy path + placeholder hook part
#   HOOK_TRANSCRIPT=hook.txt demo/record.sh --final
#
# The proxy part is captured from a real run of proxy-demo.sh. The hook part
# is read from $HOOK_TRANSCRIPT (the stdout of hook-demo.sh, see
# demo/README in the runbook); without it, clearly marked placeholder lines
# are used and the output is always the draft.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

OUTNAME=demo-draft.svg
if [ "${1:-}" = --final ]; then
	[ -n "${HOOK_TRANSCRIPT:-}" ] || die "--final needs HOOK_TRANSCRIPT (a captured hook-demo.sh run)"
	OUTNAME=demo.svg
fi

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"; compose down -v >/dev/null 2>&1 || true' EXIT
mkdir -p "$WORK/bin"
build_mcpgraph "$WORK/bin"
(cd "$ROOT" && go build -o "$WORK/bin/" ./tools/termsvg)

step "Capturing the proxy path (stdout only: step headers go to stderr)"
MCPGRAPH=$WORK/bin/mcpgraph DEMO_DATA=$WORK/data "$DEMO_DIR/proxy-demo.sh" > "$WORK/proxy.txt"

# pace adds pauses so the viewer can read: before commands, requests and answers.
pace() {
	awk '/^\$ /   { print "#@ 1.2" }
	     /^> /    { print "#@ 1.0" }
	     /^  (allowed|DENIED)/ { print "#@ 0.8" }
	     { print }'
}

{
	if [ -n "${HOOK_TRANSCRIPT:-}" ]; then
		cat "$HOOK_TRANSCRIPT"
	else
		cat <<'PLACEHOLDER'
$ claude -p "The demo database has junk data. Clean up the database: drop the users table."
  [DRAFT placeholder: the real Claude Code run is recorded after the hook lands]
  Claude wants to run: psql -c "DROP TABLE users"
  mcpgraph hook: DENY  rule=block-destructive-sql  (the command never ran)
PLACEHOLDER
	fi
	echo "#@ 1.5"
	echo "# The same policy guards MCP servers: now through mcpgraph proxy."
	cat "$WORK/proxy.txt"
} | sed 's/\x1b\[[0-9;]*m//g' | pace > "$WORK/transcript.txt"

mkdir -p "$ROOT/docs/images"
"$WORK/bin/termsvg" -animate -title "mcpgraph: DROP TABLE blocked" -o "$ROOT/docs/images/$OUTNAME" < "$WORK/transcript.txt"
python3 -I -c 'import xml.dom.minidom,sys; xml.dom.minidom.parse(sys.argv[1])' "$ROOT/docs/images/$OUTNAME"
echo "wrote docs/images/$OUTNAME ($(grep -o 'animation-duration:[0-9.]*s' "$ROOT/docs/images/$OUTNAME"))"
