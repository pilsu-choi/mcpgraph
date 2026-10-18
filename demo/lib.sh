# Shared helpers for the demo scripts (source this file; it sets no options).
DEMO_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=$(cd "$DEMO_DIR/.." && pwd)
export DEMO_PG_PORT=${DEMO_PG_PORT:-55442}
# Demo-only credentials (see docker-compose.yml); not a secret.
PG_URL="postgresql://demo:demo@127.0.0.1:$DEMO_PG_PORT/demo"
# Pinned: the official server is deprecated on npm but still works.
PG_MCP="@modelcontextprotocol/server-postgres@0.6.2"

step() { printf '\n\033[1;36m== %s\033[0m\n' "$*" >&2; }
die() { printf '\033[1;31mdemo: %s\033[0m\n' "$*" >&2; exit 1; }
compose() { docker compose -f "$DEMO_DIR/docker-compose.yml" "$@"; }
# psql runs inside the container, so the host needs no postgres client.
psql_in() { compose exec -T db psql -U demo -d demo "$@"; }

need() { command -v "$1" >/dev/null 2>&1 || die "$1 not found: $2"; }

db_up() {
	need docker "install Docker (on WSL: Docker Desktop with WSL integration enabled)"
	docker info >/dev/null 2>&1 || die "docker daemon not reachable (start Docker Desktop / enable WSL integration)"
	step "Starting the throwaway PostgreSQL (127.0.0.1:$DEMO_PG_PORT)"
	compose up -d --wait db >&2 || die "database did not become healthy; is port $DEMO_PG_PORT free? (set DEMO_PG_PORT)"
}

# table_exists succeeds when public.users is still there.
table_exists() { [ "$(psql_in -Atc "select to_regclass('public.users') is not null")" = t ]; }

# build_mcpgraph DIR builds the binary into DIR (CGO off, like releases).
build_mcpgraph() {
	need go "put Go on PATH"
	(cd "$ROOT" && CGO_ENABLED=0 go build -o "$1/" ./cmd/mcpgraph)
}
