#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/deploy/compose/docker-compose.microservices.yml"
PROJECT_NAME="${COMPOSE_PROJECT_NAME:-dipole-microservices-smoke}"
GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:8080}"

if [[ "${BUILD_IMAGE:-0}" == "1" ]]; then
  "${SCRIPT_DIR}/docker-build.sh" backend
  "${SCRIPT_DIR}/docker-build-microservice-images.sh"
fi

: "${DIPOLE_MIGRATE_IMAGE:=dipole-migrate:latest}"
: "${DIPOLE_CORE_IMAGE:=dipole-core:latest}"
: "${DIPOLE_GATEWAY_IMAGE:=dipole-gateway:latest}"
: "${DIPOLE_MESSAGE_IMAGE:=dipole-message:latest}"
: "${DIPOLE_SYNC_IMAGE:=dipole-sync:latest}"
: "${DIPOLE_SEARCH_IMAGE:=dipole-search:latest}"
: "${DIPOLE_SEARCH_INDEXER_IMAGE:=dipole-search-indexer:latest}"
: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=$(openssl rand -hex 32)}"
export DIPOLE_MIGRATE_IMAGE DIPOLE_CORE_IMAGE DIPOLE_GATEWAY_IMAGE DIPOLE_MESSAGE_IMAGE
export DIPOLE_SYNC_IMAGE DIPOLE_SEARCH_IMAGE DIPOLE_SEARCH_INDEXER_IMAGE DIPOLE_INTERNAL_RPC_SHARED_SECRET

compose() {
  docker compose -p "${PROJECT_NAME}" -f "${COMPOSE_FILE}" "$@"
}

cleanup() {
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

"${SCRIPT_DIR}/generate-internal-certs.sh"
compose config --quiet
compose up -d --wait

health=""
for _ in $(seq 1 30); do
  health="$(curl --connect-timeout 2 --max-time 5 -fsS "${GATEWAY_URL}/health" 2>/dev/null || true)"
  if [[ "${health}" == *'"component":"gateway"'* ]]; then
    break
  fi
  sleep 1
done
[[ "${health}" == *'"component":"gateway"'* ]]

proxy_status="$(curl --connect-timeout 2 --max-time 5 -sS -o /dev/null -w '%{http_code}' "${GATEWAY_URL}/api/v1/contacts" || true)"
[[ "${proxy_status}" == "401" ]]

core_ws_status="$(compose exec -T core sh -c \
  'wget -S -O /dev/null http://127.0.0.1:8081/api/v1/ws 2>&1 || true' \
  | sed -n 's/.*HTTP\/1.1 \([0-9][0-9][0-9]\).*/\1/p' \
  | head -n 1)"
[[ "${core_ws_status}" == "404" ]]

for service in core message sync gateway; do
  compose exec -T "${service}" wget -q -O - http://127.0.0.1:9100/livez | grep -qx 'alive'
  compose exec -T "${service}" wget -q -O - http://127.0.0.1:9100/readyz | grep -qx 'ready'
  compose exec -T "${service}" wget -q -O - http://127.0.0.1:9100/metrics | grep -q 'dipole_service_ready{service="dipole-'
done

echo "Microservices smoke passed: readiness, metrics, Core proxy, mTLS startup, remote WS ownership"
