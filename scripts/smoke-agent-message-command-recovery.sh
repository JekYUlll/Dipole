#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
go_bin=${DIPOLE_GO_BIN:-go}
mysql_image=${DIPOLE_AGENT_MESSAGE_RECOVERY_MYSQL_IMAGE:-mysql:8.4}
container_name=${DIPOLE_AGENT_MESSAGE_RECOVERY_CONTAINER:-"dipole-agent-message-recovery-${RANDOM}-${RANDOM}"}
mysql_password=$(openssl rand -hex 18)

command -v docker >/dev/null 2>&1 || { echo "Docker is required" >&2; exit 2; }
command -v "${go_bin}" >/dev/null 2>&1 || { echo "Go is required: ${go_bin}" >&2; exit 2; }

cleanup() {
  docker rm -f "${container_name}" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

container_id=$(docker run -d --name "${container_name}" \
  -e MYSQL_ROOT_PASSWORD="${mysql_password}" \
  -e MYSQL_DATABASE=dipole \
  -p 127.0.0.1::3306 \
  "${mysql_image}" --innodb-use-native-aio=0)

for _ in $(seq 1 90); do
  if docker exec "${container_id}" mysqladmin ping -h 127.0.0.1 -uroot -p"${mysql_password}" --silent >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "${container_id}" mysqladmin ping -h 127.0.0.1 -uroot -p"${mysql_password}" --silent >/dev/null 2>&1 || {
  echo "MySQL did not become ready" >&2
  exit 1
}

mysql_port=$(docker port "${container_id}" 3306/tcp | sed -n 's/.*:\([0-9][0-9]*\)$/\1/p' | head -n 1)
[[ -n "${mysql_port}" ]] || { echo "MySQL loopback port is unavailable" >&2; exit 1; }

cd "${root_dir}"
DIPOLE_TEST_AGENT_MESSAGE_COMMAND_MYSQL_DSN="root:${mysql_password}@tcp(127.0.0.1:${mysql_port})/dipole?parseTime=true&multiStatements=true" \
  GOTOOLCHAIN=local "${go_bin}" test ./internal/transport/grpc/message \
  -run '^TestCoreAgentMessageCommandRecoversOneMySQLCommitAfterGRPCResponseLoss$' \
  -count=1

echo "Agent Message command recovery smoke passed: MySQL is isolated and will be removed"
