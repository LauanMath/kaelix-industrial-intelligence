#!/usr/bin/env bash
# Install the Kaelix API as a macOS LaunchAgent: it starts at login, restarts if it stops
# and keeps the Mac awake on AC power. Run it again to update the service.
#
#   cp deploy/.env.example deploy/.env.service   # fill in the "Serviço nativo" section
#   ./scripts/install-launchagent.sh [environment file]
set -euo pipefail

label="com.kaelix.api"
repo_dir="$(cd "$(dirname "$0")/.." && pwd)"
env_source="${1:-$repo_dir/deploy/.env.service}"
template="$repo_dir/deploy/launchd/$label.plist.template"
data_dir="$HOME/Library/Application Support/Kaelix/api"
log_dir="$HOME/Library/Logs/Kaelix"
agents_dir="$HOME/Library/LaunchAgents"
binary="$data_dir/bin/kaelix-api"
env_file="$data_dir/service.env"
plist="$agents_dir/$label.plist"
domain="gui/$(id -u)"

fail() {
  echo "$1" >&2
  exit 1
}

[[ "$(uname -s)" == "Darwin" ]] || fail "Este instalador é só para macOS."
command -v go >/dev/null || fail "Go não encontrado no PATH."
command -v brew >/dev/null || fail "Homebrew não encontrado no PATH."
[[ -f "$template" ]] || fail "Modelo não encontrado: $template"
[[ -f "$env_source" ]] || fail "Arquivo de ambiente não encontrado: $env_source. Copie deploy/.env.example para deploy/.env.service e preencha."

# The API refuses to start without its database, and launchd would keep restarting it.
postgres_service="$(brew services list | awk '$1 ~ /^postgresql/ && $2 == "started" { print $1; exit }')"
[[ -n "$postgres_service" ]] || fail "Nenhum serviço PostgreSQL do Homebrew em execução. Inicie um, por exemplo: brew services start postgresql@17"
echo "PostgreSQL em execução: $postgres_service"

grep -Eq '^(export +)?DATABASE_URL=.+' "$env_source" || fail "Defina DATABASE_URL em $env_source."

# Another instance may hold the ports. It belongs to whoever started it: only warn.
if launchctl print "$domain/com.kaelix.api.lan" >/dev/null 2>&1; then
  echo "Aviso: com.kaelix.api.lan também está carregado. Confira se os endereços e o banco dos dois serviços não coincidem." >&2
fi

mkdir -p "$data_dir/bin" "$log_dir" "$agents_dir"
chmod 700 "$data_dir"

echo "Compilando a API..."
(cd "$repo_dir/apps/server" && GOMAXPROCS=2 taskpolicy -b go build -p 2 -trimpath -o "$binary.new" ./cmd/api)
mv "$binary.new" "$binary"

install -m 600 "$env_source" "$env_file"

for path in "$env_file" "$binary" "$data_dir" "$log_dir"; do
  [[ "$path" != *[\&\<\>\|]* ]] || fail "Caminho com caractere não suportado: $path"
done
sed -e "s|__LABEL__|$label|g" -e "s|__ENV_FILE__|$env_file|g" -e "s|__API_BINARY__|$binary|g" -e "s|__DATA_DIR__|$data_dir|g" -e "s|__LOG_DIR__|$log_dir|g" "$template" >"$plist.new"
plutil -lint "$plist.new" >/dev/null || fail "O plist gerado é inválido: $plist.new"
mv "$plist.new" "$plist"

launchctl bootout "$domain/$label" >/dev/null 2>&1 || true
launchctl bootstrap "$domain" "$plist"

# shellcheck disable=SC1090
address="$(set -a; . "$env_file"; echo "${API_ADDR:-127.0.0.1:8080}")"
for _ in $(seq 1 20); do
  if curl --noproxy '*' --silent --fail "http://$address/healthz" >/dev/null; then
    echo "API ativa em http://$address, serviço $label."
    echo "Logs: $log_dir/api.log"
    echo "Parar: launchctl bootout $domain/$label"
    exit 0
  fi
  sleep 1
done
fail "A API não respondeu em http://$address. Consulte $log_dir/api.log."
