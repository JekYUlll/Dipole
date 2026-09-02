#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${DIPOLE_COMPOSE_FILE:-${ROOT_DIR}/deploy/compose/docker-compose.microservices.yml}"
PROJECT_NAME="${COMPOSE_PROJECT_NAME:-dipole-microservices-lite}"
GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:8080}"

: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=$(openssl rand -hex 32)}"
export DIPOLE_INTERNAL_RPC_SHARED_SECRET

compose() {
  docker compose -p "${PROJECT_NAME}" -f "${COMPOSE_FILE}" "$@"
}

cleanup() {
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

"${SCRIPT_DIR}/check-dev-host.sh" "${DIPOLE_HOST_PROFILE:-tencent-cloud}"
"${SCRIPT_DIR}/generate-internal-certs.sh"
compose config --quiet
compose up -d --wait gateway

health=""
for _ in $(seq 1 30); do
  health="$(curl --connect-timeout 2 --max-time 5 -fsS "${GATEWAY_URL}/health" 2>/dev/null || true)"
  if [[ "${health}" == *'"component":"gateway"'* ]]; then
    break
  fi
  sleep 1
done
[[ "${health}" == *'"component":"gateway"'* ]]

proxy_status="$(curl --connect-timeout 2 --max-time 5 -sS -o /dev/null -w '%{http_code}' \
  "${GATEWAY_URL}/api/v1/contacts" || true)"
[[ "${proxy_status}" == "401" ]]

for service in core message sync gateway; do
  compose exec -T "${service}" wget -q -O - http://127.0.0.1:9100/livez | grep -qx 'alive'
  compose exec -T "${service}" wget -q -O - http://127.0.0.1:9100/readyz | grep -qx 'ready'
done

running_services="$(compose ps --status running --services | sort)"
expected_services=$'core\ngateway\nkafka\nmessage\nminio\nmysql\nredis\nsync'
[[ "${running_services}" == "${expected_services}" ]]

if compose ps --status running --services | grep -Eq '^(agent|search|search-indexer|realtime-cpp)$'; then
  echo "optional services must remain stopped in the lightweight smoke" >&2
  exit 1
fi

echo "Lightweight microservices smoke passed: gateway/core/message/sync readiness, auth proxy, and optional-service isolation"
