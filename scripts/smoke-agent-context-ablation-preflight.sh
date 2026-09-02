#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
go_bin=${DIPOLE_GO_BIN:-go}
project="dipole-agent-context-ablation-preflight-${RANDOM}-$$"
network="${project}-network"
container="${project}-mysql"
work_dir=$(mktemp -d /tmp/dipole-agent-context-ablation.XXXXXX)
config_file="${work_dir}/config.yaml"

cleanup() {
  local status=$?
  docker rm -f "$container" >/dev/null 2>&1 || true
  docker network rm "$network" >/dev/null 2>&1 || true
  rm -rf "$work_dir"
  exit "$status"
}
trap cleanup EXIT INT TERM

docker network create "$network" >/dev/null
docker run -d --name "$container" --network "$network" --network-alias mysql \
  -e MYSQL_ROOT_PASSWORD=ablation-root -e MYSQL_ROOT_HOST=% -e MYSQL_DATABASE=dipole \
  -p 127.0.0.1::3306 mysql:8.4 >/dev/null

for _ in $(seq 1 90); do
  docker exec "$container" mysqladmin ping -h 127.0.0.1 -uroot -pablation-root --silent >/dev/null 2>&1 && break
  sleep 1
done
docker exec "$container" mysqladmin ping -h 127.0.0.1 -uroot -pablation-root --silent >/dev/null
port=$(docker port "$container" 3306/tcp | sed -n 's/.*:\([0-9][0-9]*\)$/\1/p')
[[ -n "$port" ]]

cat >"$config_file" <<YAML
mysql:
  host: 127.0.0.1
  port: ${port}
  user: root
  password: ablation-root
  dbname: dipole
YAML

(
  cd "$root_dir"
  CGO_ENABLED=0 "$go_bin" build -o "$work_dir/dipole-migrate" ./cmd/tools/migrate
)
DIPOLE_CONFIG_FILE="$config_file" DIPOLE_MYSQL_HOST=127.0.0.1 DIPOLE_MYSQL_PORT="$port" \
  DIPOLE_MYSQL_USER=root DIPOLE_MYSQL_PASSWORD=ablation-root DIPOLE_MYSQL_DBNAME=dipole \
  "$work_dir/dipole-migrate" -direction up >/dev/null

docker exec -i "$container" mysql -uroot -pablation-root <"$root_dir/configs/mysql/agent-eval-grants.dist.sql"
mapfile -t checks < <(docker exec "$container" mysql -N -B -uroot -pablation-root dipole -e "
  SELECT COUNT(*) FROM schema_migrations WHERE version = 56;
  SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'dipole' AND table_name = 'agent_context_ablation_bindings';
  SELECT COUNT(*) FROM information_schema.table_privileges
    WHERE table_schema = 'dipole' AND table_name = 'agent_context_ablation_bindings'
      AND grantee = CONCAT(CHAR(39), 'dipole_agent_eval', CHAR(39), '@', CHAR(39), '%', CHAR(39))
      AND privilege_type IN ('INSERT', 'UPDATE', 'DELETE');")
[[ "${#checks[@]}" == "3" && "${checks[0]}" == "1" && "${checks[1]}" == "1" && "${checks[2]}" == "0" ]]

read_count=$(docker exec "$container" mysql -N -B -udipole_agent_eval -pchange-me dipole \
  -e "SELECT COUNT(*) FROM agent_context_ablation_bindings;")
[[ "$read_count" == "0" ]]

printf 'Context Ablation preflight passed: migration 000056, binding table, and read-only Eval account are ready.\n'
