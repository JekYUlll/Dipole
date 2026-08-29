#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root_dir=$(cd "${script_dir}/.." && pwd)
project="${COMPOSE_PROJECT_NAME:-dipole-sync-cassandra-primary-${RANDOM}-$$}"
compose_file="${root_dir}/deploy/compose/docker-compose.microservices.yml"
primary_file="${root_dir}/deploy/microservices/cassandra-primary.yml"

: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=$(openssl rand -hex 32)}"
export DIPOLE_INTERNAL_RPC_SHARED_SECRET
if [[ -z "${DIPOLE_INTERNAL_CERT_DIR:-}" ]]; then
  export DIPOLE_INTERNAL_CERT_DIR="${root_dir}/certs/internal"
fi

compose() {
  docker compose -p "${project}" --profile cassandra-primary \
    -f "${compose_file}" -f "${primary_file}" "$@"
}

cleanup() {
  local exit_code=$?
  if [[ "${KEEP_STACK:-0}" == "1" && "${exit_code}" != "0" ]]; then
    printf 'Sync Cassandra primary Compose stack retained: project=%s\n' "${project}" >&2
  else
    compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  fi
  exit "${exit_code}"
}
trap cleanup EXIT INT TERM

"${script_dir}/generate-internal-certs.sh" >/dev/null
compose config --quiet
compose up -d --wait --wait-timeout "${CASSANDRA_PRIMARY_READY_TIMEOUT_SECONDS:-180}" cassandra-init message sync

ready=$(compose exec -T sync wget -q -O - http://127.0.0.1:9100/readyz)
test "${ready}" = ready
metrics=$(compose exec -T sync wget -q -O - http://127.0.0.1:9100/metrics)
grep -q 'dipole_service_ready{service="dipole-sync"} 1' <<<"${metrics}"

printf 'Sync Cassandra primary Compose smoke passed: project=%s primary=true schema-init=true ready=true\n' "${project}"
