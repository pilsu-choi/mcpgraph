#!/usr/bin/env bash
# Path A of the demo: a real PostgreSQL MCP server behind `mcpgraph proxy`.
#
#   MCPGRAPH=/path/to/mcpgraph DEMO_DATA=/path/to/data demo/proxy-demo.sh
#
# Plays the part of the AI app (an MCP client): initialize, list tools, run a
# harmless query, then try DROP TABLE. mcpgraph answers the DROP itself, so
# the server never sees it. Afterwards it lists the audit events and checks
# with psql that the table is still there.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

MCPGRAPH=${MCPGRAPH:?set MCPGRAPH to the mcpgraph binary}
DEMO_DATA=${DEMO_DATA:?set DEMO_DATA to a throwaway data dir}
need npx "install Node.js (the MCP server runs with npx)"
need python3 "needed to pretty-print JSON-RPC"

db_up
OUT=$(mktemp)
FIFO_DIR=$(mktemp -d)
trap 'rm -rf "$OUT" "$FIFO_DIR"; [ -z "${PROXY_PID:-}" ] || kill "$PROXY_PID" 2>/dev/null || true' EXIT

step "Start the MCP server behind mcpgraph proxy"
echo "# MCP server: Postgres. Every tools/call goes through mcpgraph first."
echo "\$ mcpgraph proxy --server postgres --app \"Claude Desktop\" -- npx -y $PG_MCP <postgres url>"
mkfifo "$FIFO_DIR/in"
"$MCPGRAPH" proxy --data-dir "$DEMO_DATA" --server postgres --app "Claude Desktop" \
	-- npx -y "$PG_MCP" "$PG_URL" < "$FIFO_DIR/in" > "$OUT" 2>"$FIFO_DIR/err" &
PROXY_PID=$!
exec 3> "$FIFO_DIR/in" # keep the proxy's stdin open until we close fd 3

# send ID JSON: writes one request, waits (max 90 s) for the answer with that ID.
send() {
	local id=$1 msg=$2
	printf '%s\n' "$msg" >&3
	for _ in $(seq 900); do
		grep -qE "\"id\":$id[,}]" "$OUT" && return 0
		sleep 0.1
	done
	cat "$FIFO_DIR/err" >&2
	die "no answer for request $id"
}
# show ID prints the answer to request ID, short form.
show() {
	grep -E "\"id\":$1[,}]" "$OUT" | python3 -I -c '
import json, sys
m = json.loads(sys.stdin.readline())
r = m.get("result", {})
if "tools" in r:
    print("  tools:", ", ".join(t["name"] for t in r["tools"]))
elif "serverInfo" in r:
    print("  initialized:", r["serverInfo"]["name"])
elif "content" in r:
    err = r.get("isError", False)
    meta = r.get("_meta", {})
    tag = "DENIED  rule=%s" % meta.get("mcpgraph/rule_id") if err else "allowed"
    print("  %s -> %s" % (tag, r["content"][0]["text"].replace("\n", " ")[:100]))
else:
    print(" ", m)
'
}

step "MCP client: initialize and list tools"
send 1 '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"demo-client","version":"1.0"}}}'
show 1
printf '%s\n' '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&3
send 2 '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'
show 2

step "Allowed: a harmless query"
echo "# a harmless query is allowed and forwarded"
echo '> tools/call query {"sql":"SELECT count(*) FROM users"}'
send 3 '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"query","arguments":{"sql":"SELECT count(*) FROM users"}}}'
show 3

step "Blocked: the agent tries to clean up the database"
echo "# the agent tries to clean up the database"
echo '> tools/call query {"sql":"DROP TABLE users"}'
send 4 '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"query","arguments":{"sql":"DROP TABLE users"}}}'
show 4

exec 3>&- # EOF on stdin: the proxy and the server shut down
wait "$PROXY_PID" 2>/dev/null || true
PROXY_PID=

step "Audit log"
echo "# every decision is in the audit log"
echo "\$ mcpgraph events"
"$MCPGRAPH" events --data-dir "$DEMO_DATA"

step "Proof: the table is still there"
echo "# and the table is still there"
echo "\$ psql -c 'select count(*) from users'   # inside the container"
psql_in -c "select count(*) from users"
table_exists || die "users table is gone!"
echo "users table still exists."
