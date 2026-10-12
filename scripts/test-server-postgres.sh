#!/usr/bin/env bash
# Start a disposable, socket-only PostgreSQL cluster and run real database tests.
set -euo pipefail
repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
if [[ -n "${TEST_DATABASE_URL:-}" ]]; then
  cd "$repo_dir/apps/server"
  go test -race -count=1 ./...
  exit
fi
for tool in initdb pg_ctl; do
  if ! command -v "$tool" >/dev/null; then
    echo "Install PostgreSQL tools or set TEST_DATABASE_URL to a dedicated test database." >&2
    exit 1
  fi
done
cluster_dir="$(mktemp -d /tmp/kaelix-pg.XXXXXX)"
cleanup() {
  pg_ctl -D "$cluster_dir/data" -m immediate -w stop >/dev/null 2>&1 || true
  rm -rf "$cluster_dir"
}
trap cleanup EXIT
initdb -D "$cluster_dir/data" -U postgres -A trust --no-locale >/dev/null
pg_ctl -D "$cluster_dir/data" -l "$cluster_dir/postgres.log" -o "-h '' -k $cluster_dir -p 55432" -w start >/dev/null
export TEST_DATABASE_URL="postgres://postgres@localhost/postgres?host=$cluster_dir&port=55432&sslmode=disable"
cd "$repo_dir/apps/server"
go test -race -count=1 ./...
