#!/usr/bin/env bash
set -euo pipefail

# Verifies the development-only Prometheus -> Alertmanager transport path for
# Multipart alerts. A generated alert proves routing while the checked-in rules
# remain mounted and parsed by the same Prometheus process.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${DIPOLE_COMPOSE_FILE:-${ROOT_DIR}/deploy/compose/docker-compose.microservices.yml}"
PROJECT_NAME="${COMPOSE_PROJECT_NAME:-dipole-multipart-alertmanager-${RANDOM}-$$}"
timeout_seconds="${DIPOLE_MULTIPART_ALERTMANAGER_SMOKE_TIMEOUT_SECONDS:-90}"
scratch_dir="$(mktemp -d "${TMPDIR:-/tmp}/dipole-multipart-alertmanager.XXXXXX")"
prometheus_config="${scratch_dir}/prometheus-services.yml"
smoke_rule="${scratch_dir}/multipart-alertmanager-smoke.yml"
compose_override="${scratch_dir}/compose.override.yml"
alertmanager_config="${ROOT_DIR}/deploy/observability/alertmanager.yml"

command -v docker >/dev/null 2>&1 || { printf 'Docker is required\n' >&2; exit 2; }
command -v timeout >/dev/null 2>&1 || { printf 'timeout is required\n' >&2; exit 2; }
command -v openssl >/dev/null 2>&1 || { printf 'openssl is required\n' >&2; exit 2; }
[[ -f "${COMPOSE_FILE}" ]] || { printf 'Compose file does not exist: %s\n' "${COMPOSE_FILE}" >&2; exit 2; }
[[ -f "${alertmanager_config}" ]] || { printf 'Alertmanager config does not exist: %s\n' "${alertmanager_config}" >&2; exit 2; }
[[ "${timeout_seconds}" =~ ^[0-9]+$ ]] && (( timeout_seconds >= 30 && timeout_seconds <= 300 )) || {
  printf 'DIPOLE_MULTIPART_ALERTMANAGER_SMOKE_TIMEOUT_SECONDS must be between 30 and 300 seconds\n' >&2
  exit 2
}

: "${DIPOLE_PROMETHEUS_BIND_ADDRESS:=127.0.0.1}"
: "${DIPOLE_PROMETHEUS_PORT:=$((20000 + RANDOM % 1000))}"
: "${DIPOLE_ALERTMANAGER_BIND_ADDRESS:=127.0.0.1}"
: "${DIPOLE_ALERTMANAGER_PORT:=$((21000 + RANDOM % 1000))}"
: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=$(openssl rand -hex 32)}"
: "${DIPOLE_AGENT_MODEL_PROVIDER_NAME:=multipart-alertmanager-smoke}"
: "${DIPOLE_AGENT_MODEL_BASE_URL:=https://models.invalid/v1}"
: "${DIPOLE_AGENT_MODEL_API_KEY:=multipart-alertmanager-smoke-no-network}"
: "${DIPOLE_AGENT_MODEL_ROUTES:=multipart-alertmanager-smoke/deterministic}"
: "${DIPOLE_AGENT_MODEL_CONTEXT_PROFILES:=[{\"route\":\"multipart-alertmanager-smoke/deterministic\",\"contextWindowTokens\":32768,\"utf8BytesPerToken\":3,\"safetyMarginBps\":1500}]}"
export DIPOLE_PROMETHEUS_BIND_ADDRESS DIPOLE_PROMETHEUS_PORT
export DIPOLE_ALERTMANAGER_BIND_ADDRESS DIPOLE_ALERTMANAGER_PORT
export DIPOLE_INTERNAL_RPC_SHARED_SECRET
export DIPOLE_AGENT_MODEL_PROVIDER_NAME DIPOLE_AGENT_MODEL_BASE_URL DIPOLE_AGENT_MODEL_API_KEY
export DIPOLE_AGENT_MODEL_ROUTES DIPOLE_AGENT_MODEL_CONTEXT_PROFILES
export DIPOLE_MULTIPART_ALERTMANAGER_SMOKE_DIR="${scratch_dir}"

cat >"${prometheus_config}" <<'YAML'
global:
  scrape_interval: 5s
  evaluation_interval: 1s

alerting:
  alertmanagers:
    - static_configs:
        - targets: [alertmanager:9093]

rule_files:
  - /etc/prometheus/multipart-alerts.yml
  - /smoke/multipart-alertmanager-smoke.yml

scrape_configs: []
YAML

cat >"${smoke_rule}" <<'YAML'
groups:
  - name: dipole-multipart-alertmanager-smoke
    interval: 1s
    rules:
      - alert: DipoleMultipartAlertmanagerRoutingSmoke
        expr: vector(1)
        labels:
          severity: info
        annotations:
          summary: Isolated Multipart alert routing smoke
YAML

cat >"${compose_override}" <<'YAML'
services:
  prometheus:
    command: ["--config.file=/smoke/prometheus-services.yml"]
    volumes:
      - ${DIPOLE_MULTIPART_ALERTMANAGER_SMOKE_DIR}:/smoke:ro
YAML

# Prometheus runs as an unprivileged container user, so the generated config
# needs to be readable without exposing any credentials or host state.
chmod 755 "${scratch_dir}"
chmod 644 "${prometheus_config}" "${smoke_rule}" "${compose_override}"

compose() {
  docker compose -p "${PROJECT_NAME}" -f "${COMPOSE_FILE}" -f "${compose_override}" "$@"
}

cleanup() {
  local status=$?
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    compose --profile observability down -v --remove-orphans >/dev/null 2>&1 || true
    rm -rf "${scratch_dir}"
  else
    printf 'Multipart Alertmanager smoke stack retained: project=%s scratch=%s\n' "${PROJECT_NAME}" "${scratch_dir}" >&2
  fi
  exit "${status}"
}
trap cleanup EXIT INT TERM

compose --profile observability config --quiet
timeout --preserve-status "${timeout_seconds}s" docker compose -p "${PROJECT_NAME}" -f "${COMPOSE_FILE}" -f "${compose_override}" --profile observability up -d --wait prometheus alertmanager

for _ in $(seq 1 "${timeout_seconds}"); do
  alerts="$(compose exec -T alertmanager wget -q -O - http://127.0.0.1:9093/api/v2/alerts 2>/dev/null || true)"
  if grep -q '"alertname":"DipoleMultipartAlertmanagerRoutingSmoke"' <<<"${alerts}"; then
    printf 'Multipart Alertmanager routing smoke passed: synthetic Multipart alert reached the development discard receiver.\n'
    printf 'Checked-in multipart rules were mounted and parsed; this does not validate a production receiver or change multipart_mode.\n'
    exit 0
  fi
  sleep 1
done

printf 'Timed out waiting for the synthetic Multipart alert in Alertmanager\n' >&2
exit 1
