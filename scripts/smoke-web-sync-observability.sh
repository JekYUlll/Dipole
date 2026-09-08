#!/usr/bin/env bash
set -euo pipefail

# Prepare an isolated Prometheus preflight before opening a real 24-hour Web
# Sync observation window. It never enables a client sync mode or promotes one.
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root_dir="$(cd "${script_dir}/.." && pwd)"
compose_file="${DIPOLE_COMPOSE_FILE:-${root_dir}/deploy/compose/docker-compose.microservices.yml}"
project_name="${COMPOSE_PROJECT_NAME:-dipole-web-sync-observability}"
gateway_port="${DIPOLE_GATEWAY_PORT:-18080}"
prometheus_port="${DIPOLE_PROMETHEUS_PORT:-19090}"
alertmanager_port="${DIPOLE_ALERTMANAGER_PORT:-19093}"
startup_timeout_seconds="${DIPOLE_WEB_SYNC_OBSERVABILITY_STARTUP_TIMEOUT_SECONDS:-300}"
env_file="${DIPOLE_ENV_FILE:-}"

if ! [[ "${startup_timeout_seconds}" =~ ^[0-9]+$ ]] || (( startup_timeout_seconds < 30 || startup_timeout_seconds > 1800 )); then
  echo "Web Sync observability startup timeout must be between 30 and 1800 seconds" >&2
  exit 2
fi
if ! command -v timeout >/dev/null 2>&1; then
  echo "Web Sync observability smoke requires the timeout command" >&2
  exit 2
fi
if [[ -n "${env_file}" && ! -r "${env_file}" ]]; then
  echo "Web Sync observability env file must be readable when DIPOLE_ENV_FILE is set" >&2
  exit 2
fi

: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=$(openssl rand -hex 32)}"
export DIPOLE_INTERNAL_RPC_SHARED_SECRET
export DIPOLE_GATEWAY_BIND_ADDRESS="${DIPOLE_GATEWAY_BIND_ADDRESS:-127.0.0.1}"
export DIPOLE_GATEWAY_PORT="${gateway_port}"
export DIPOLE_PROMETHEUS_BIND_ADDRESS="${DIPOLE_PROMETHEUS_BIND_ADDRESS:-127.0.0.1}"
export DIPOLE_PROMETHEUS_PORT="${prometheus_port}"
export DIPOLE_ALERTMANAGER_BIND_ADDRESS="${DIPOLE_ALERTMANAGER_BIND_ADDRESS:-127.0.0.1}"
export DIPOLE_ALERTMANAGER_PORT="${alertmanager_port}"

compose_command=(docker compose)
if [[ -n "${env_file}" ]]; then
  compose_command+=(--env-file "${env_file}")
fi
compose_command+=(-p "${project_name}" -f "${compose_file}")

compose() {
  "${compose_command[@]}" "$@"
}

require_image_revision() {
  local image="$1"
  local expected_revision="$2"
  local actual_revision
  actual_revision="$(docker image inspect "${image}" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' 2>/dev/null || true)"
  if [[ "${actual_revision}" != "${expected_revision}" ]]; then
    echo "Web Sync observability image ${image} revision ${actual_revision:-missing} does not match ${expected_revision}; build migrate core message sync gateway from this checkout first" >&2
    return 1
  fi
}

require_image_revisions() {
  local revision
  revision="$(git -C "${root_dir}" rev-parse HEAD)"
  require_image_revision "${DIPOLE_MIGRATE_IMAGE:-dipole-migrate:latest}" "${revision}"
  require_image_revision "${DIPOLE_CORE_IMAGE:-dipole-core:latest}" "${revision}"
  require_image_revision "${DIPOLE_MESSAGE_IMAGE:-dipole-message:latest}" "${revision}"
  require_image_revision "${DIPOLE_SYNC_IMAGE:-dipole-sync:latest}" "${revision}"
  require_image_revision "${DIPOLE_GATEWAY_IMAGE:-dipole-gateway:latest}" "${revision}"
}

cleanup() {
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    compose --profile observability down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

required_targets_are_healthy() {
  python3 -c '
import json
import sys

required = {"dipole-core", "dipole-message", "dipole-sync", "dipole-gateway"}
payload = json.load(sys.stdin)
targets = payload.get("data", {}).get("activeTargets", [])
health = {
    target.get("labels", {}).get("service"): target.get("health")
    for target in targets
    if target.get("labels", {}).get("service") in required
}
sys.exit(0 if all(health.get(service) == "up" for service in required) else 1)
' <<<"$1"
}

wait_for_healthy_targets() {
  local targets=""
  for _ in $(seq 1 30); do
    targets="$(curl --connect-timeout 2 --max-time 5 -fsS "http://127.0.0.1:${prometheus_port}/api/v1/targets?state=active&scrapePool=dipole-required")"
    if required_targets_are_healthy "${targets}"; then
      return 0
    fi
    sleep 1
  done
  printf '%s\n' "${targets}" >&2
  return 1
}

rules_are_loaded() {
  curl --connect-timeout 2 --max-time 5 -fsS "http://127.0.0.1:${prometheus_port}/api/v1/rules" |
    python3 -c '
import json
import sys

required = {
    "dipole:web_sync_shadow:matches_24h",
    "dipole:web_sync_shadow:terminal_differences_24h",
    "dipole:web_sync_shadow:overflows_24h",
    "dipole:web_sync_shadow:window_complete",
    "dipole:web_sync_shadow:promotion_ready",
}
payload = json.load(sys.stdin)
names = {
    rule.get("name")
    for group in payload.get("data", {}).get("groups", [])
    for rule in group.get("rules", [])
}
sys.exit(0 if required.issubset(names) else 1)
'
}

"${script_dir}/check-dev-host.sh" "${DIPOLE_HOST_PROFILE:-remote-gpu}"
"${script_dir}/generate-internal-certs.sh"
require_image_revisions
compose --profile observability config --quiet
timeout --preserve-status "${startup_timeout_seconds}s" "${compose_command[@]}" --profile observability up -d --wait core message sync gateway prometheus alertmanager

for _ in $(seq 1 30); do
  if curl --connect-timeout 2 --max-time 5 -fsS "http://127.0.0.1:${prometheus_port}/-/ready" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl --connect-timeout 2 --max-time 5 -fsS "http://127.0.0.1:${prometheus_port}/-/ready" >/dev/null
for _ in $(seq 1 30); do
  if curl --connect-timeout 2 --max-time 5 -fsS "http://127.0.0.1:${alertmanager_port}/-/ready" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl --connect-timeout 2 --max-time 5 -fsS "http://127.0.0.1:${alertmanager_port}/-/ready" >/dev/null
curl --connect-timeout 2 --max-time 5 -fsS "http://127.0.0.1:${gateway_port}/health" | grep -q '"component":"gateway"'
wait_for_healthy_targets
rules_are_loaded

echo "Web Sync observability preflight passed: gateway=127.0.0.1:${gateway_port} prometheus=127.0.0.1:${prometheus_port} alertmanager=127.0.0.1:${alertmanager_port}"
echo "This preflight does not enable a Web Sync client mode or start a promotion observation window."
