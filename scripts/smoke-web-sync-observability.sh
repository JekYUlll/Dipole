#!/usr/bin/env bash
set -euo pipefail

# Verify the isolated service and Prometheus wiring required before an owner can
# start a real Web Sync shadow observation window.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${DIPOLE_COMPOSE_FILE:-${ROOT_DIR}/deploy/compose/docker-compose.microservices.yml}"
PROJECT_NAME="${COMPOSE_PROJECT_NAME:-dipole-web-sync-observability}"
GATEWAY_PORT="${DIPOLE_GATEWAY_PORT:-8080}"
PROMETHEUS_PORT="${DIPOLE_PROMETHEUS_PORT:-9090}"
ALERTMANAGER_PORT="${DIPOLE_ALERTMANAGER_PORT:-9093}"
GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:${GATEWAY_PORT}}"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://127.0.0.1:${PROMETHEUS_PORT}}"
ALERTMANAGER_URL="${ALERTMANAGER_URL:-http://127.0.0.1:${ALERTMANAGER_PORT}}"
startup_timeout_seconds="${DIPOLE_WEB_SYNC_OBSERVABILITY_STARTUP_TIMEOUT_SECONDS:-300}"

if ! [[ "${startup_timeout_seconds}" =~ ^[0-9]+$ ]] || (( startup_timeout_seconds < 30 || startup_timeout_seconds > 1800 )); then
  echo "Web Sync observability startup timeout must be between 30 and 1800 seconds" >&2
  exit 2
fi
if ! command -v timeout >/dev/null 2>&1; then
  echo "Web Sync observability smoke requires the timeout command" >&2
  exit 2
fi

: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=$(openssl rand -hex 32)}"
export DIPOLE_INTERNAL_RPC_SHARED_SECRET
export DIPOLE_GATEWAY_BIND_ADDRESS="${DIPOLE_GATEWAY_BIND_ADDRESS:-127.0.0.1}"
export DIPOLE_PROMETHEUS_BIND_ADDRESS="${DIPOLE_PROMETHEUS_BIND_ADDRESS:-127.0.0.1}"
export DIPOLE_ALERTMANAGER_BIND_ADDRESS="${DIPOLE_ALERTMANAGER_BIND_ADDRESS:-127.0.0.1}"

compose() {
  docker compose -p "${PROJECT_NAME}" -f "${COMPOSE_FILE}" "$@"
}

cleanup() {
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

"${SCRIPT_DIR}/check-dev-host.sh" "${DIPOLE_HOST_PROFILE:-remote-gpu}"
"${SCRIPT_DIR}/generate-internal-certs.sh"
compose --profile observability config --quiet
timeout --preserve-status "${startup_timeout_seconds}s" docker compose -p "${PROJECT_NAME}" -f "${COMPOSE_FILE}" --profile observability up -d --wait gateway prometheus alertmanager

for _ in $(seq 1 30); do
  if curl --connect-timeout 2 --max-time 5 -fsS "${PROMETHEUS_URL}/-/ready" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl --connect-timeout 2 --max-time 5 -fsS "${PROMETHEUS_URL}/-/ready" >/dev/null
for _ in $(seq 1 30); do
  if curl --connect-timeout 2 --max-time 5 -fsS "${ALERTMANAGER_URL}/-/ready" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl --connect-timeout 2 --max-time 5 -fsS "${ALERTMANAGER_URL}/-/ready" >/dev/null
curl --connect-timeout 2 --max-time 5 -fsS "${GATEWAY_URL}/health" | grep -q '"component":"gateway"'

for service in core message sync gateway; do
  compose exec -T "${service}" wget -q -O - http://127.0.0.1:9100/metrics | grep -q '^#'
done

targets="$(curl --connect-timeout 2 --max-time 5 -fsS "${PROMETHEUS_URL}/api/v1/targets?state=active")"
for service in dipole-core dipole-message dipole-sync dipole-gateway; do
  grep -q "\"service\":\"${service}\"" <<<"${targets}"
done
if grep -Eq '"health":"(down|unknown)"' <<<"${targets}"; then
  echo "Prometheus has an unhealthy active target" >&2
  exit 1
fi

echo "Web Sync observability smoke passed: loopback gateway=${GATEWAY_URL} prometheus=${PROMETHEUS_URL} alertmanager=${ALERTMANAGER_URL}"
echo "This smoke does not start a Web Sync promotion observation window."
