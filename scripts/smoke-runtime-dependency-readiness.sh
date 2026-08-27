#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root_dir=$(cd "${script_dir}/.." && pwd)
compose_file="${root_dir}/docker-compose.microservices.yml"
project_name="${COMPOSE_PROJECT_NAME:-dipole-readiness-${RANDOM}-$$}"

if [[ "${BUILD_IMAGE:-0}" == "1" ]]; then
  image_name="${IMAGE_NAME:-dipole-server}"
  image_tag="${IMAGE_TAG:-runtime-readiness-smoke}"
  IMAGE_NAME="${image_name}" IMAGE_TAG="${image_tag}" "${script_dir}/docker-build.sh" build
  export DIPOLE_IMAGE="${image_name}:${image_tag}"
fi

: "${DIPOLE_IMAGE:=dipole-server:latest}"
: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=$(openssl rand -hex 32)}"
export DIPOLE_IMAGE DIPOLE_INTERNAL_RPC_SHARED_SECRET
export DIPOLE_SEARCH_ENABLED=true

compose() {
  docker compose -p "${project_name}" -f "${compose_file}" --profile search "$@"
}

cleanup() {
  local exit_code=$?
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  else
    printf 'Runtime readiness stack retained: project=%s\n' "${project_name}"
  fi
  exit "${exit_code}"
}
trap cleanup EXIT INT TERM

readiness_body() {
  local service=$1
  compose exec -T "${service}" sh -ec \
    'wget -q -O - http://127.0.0.1:9100/metrics 2>/dev/null | awk '\''/^dipole_service_ready\{/ {print ($2 == 1 ? "ready" : "not ready"); exit}'\''' \
    2>/dev/null | tr -d '\r' || true
}

wait_for_readiness() {
  local service=$1
  local expected=$2
  local attempts=${3:-30}
  local actual=""
  for ((attempt = 1; attempt <= attempts; attempt++)); do
    actual=$(readiness_body "${service}")
    if [[ "${actual}" == "${expected}" ]]; then
      return 0
    fi
    sleep 2
  done
  printf '%s readiness remained %q, expected %q\n' "${service}" "${actual}" "${expected}" >&2
  return 1
}

assert_dependency_ready() {
  local service=$1
  local dependency=$2
  compose exec -T "${service}" sh -ec \
    "wget -q -O - http://127.0.0.1:9100/metrics | grep -F 'dipole_dependency_ready{dependency=\"${dependency}\",service=\"dipole-${service}\"} 1'" \
    >/dev/null
}

assert_container_ids_unchanged() {
  local service
  local current
  for service in "${application_services[@]}"; do
    current=$(compose ps -q "${service}")
    if [[ -z "${current}" || "${current}" != "${container_ids[${service}]}" ]]; then
      printf '%s container changed: before=%s after=%s\n' \
        "${service}" "${container_ids[${service}]}" "${current}" >&2
      return 1
    fi
  done
}

"${script_dir}/generate-internal-certs.sh"
compose config --quiet
compose up -d \
  mysql redis kafka minio minio-init elasticsearch migrate mysql-permissions \
  core message sync search-indexer

application_services=(core message sync gateway search search-indexer)
required_services=(core message sync gateway)
declare -A container_ids
for service in core message sync search-indexer; do
  wait_for_readiness "${service}" "ready" 60
done
compose up -d search
wait_for_readiness search "ready" 60
compose up -d gateway
wait_for_readiness gateway "ready" 60
assert_dependency_ready gateway kafka-assignment
for service in "${application_services[@]}"; do
  container_ids[${service}]=$(compose ps -q "${service}")
done

printf 'Stopping Elasticsearch to exercise cached failure hysteresis\n'
compose stop elasticsearch >/dev/null
wait_for_readiness search "not ready" 20
wait_for_readiness search-indexer "not ready" 20
for service in "${required_services[@]}"; do
  wait_for_readiness "${service}" "ready" 1
done
assert_container_ids_unchanged

printf 'Restarting Elasticsearch to exercise recovery hysteresis\n'
compose start elasticsearch >/dev/null
for ((attempt = 1; attempt <= 60; attempt++)); do
  if compose exec -T elasticsearch curl -fsS http://127.0.0.1:9200/_cluster/health >/dev/null 2>&1; then
    break
  fi
  if [[ "${attempt}" == "60" ]]; then
    printf 'Elasticsearch did not recover\n' >&2
    exit 1
  fi
  sleep 2
done
wait_for_readiness search "ready" 20
wait_for_readiness search-indexer "ready" 20
for service in "${required_services[@]}"; do
  wait_for_readiness "${service}" "ready" 1
done
assert_container_ids_unchanged

printf 'Runtime dependency readiness smoke passed: Gateway assignment was established and Elasticsearch isolated Search traffic without application restart cascades.\n'
