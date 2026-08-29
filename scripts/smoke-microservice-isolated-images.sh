#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root_dir=$(cd "${script_dir}/.." && pwd)
project=${COMPOSE_PROJECT_NAME:-dipole-isolated-image-smoke}
gateway_port=${GATEWAY_PORT:-18080}
cert_dir=${DIPOLE_INTERNAL_CERT_DIR:-$(mktemp -d -t dipole-isolated-certs.XXXXXX)}
ports_file=$(mktemp -t dipole-isolated-ports.XXXXXX.yml)
remove_cert_dir=0
if [[ -z "${DIPOLE_INTERNAL_CERT_DIR:-}" ]]; then
  remove_cert_dir=1
fi

cat >"${ports_file}" <<EOF
services:
  gateway:
    ports: !override
      - "${gateway_port}:8080"
EOF

export DIPOLE_INTERNAL_CERT_DIR="${cert_dir}"
: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=$(openssl rand -hex 32)}"
export DIPOLE_INTERNAL_RPC_SHARED_SECRET

compose() {
  local -a profile_args=()
  if [[ "${SMOKE_SEARCH_PROFILE:-0}" == "1" ]]; then
    profile_args+=(--profile search)
  fi
  docker compose -p "${project}" \
    "${profile_args[@]}" \
    -f "${root_dir}/docker-compose.microservices.yml" \
    -f "${root_dir}/deploy/microservices/isolated-images.yml" \
    -f "${ports_file}" "$@"
}

cleanup() {
  local exit_code=$?
  if [[ "${KEEP_ON_FAILURE:-0}" != "1" || "${exit_code}" == "0" ]]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  else
    printf 'isolated microservices smoke retained failed project: %s\n' "${project}" >&2
  fi
  rm -f "${ports_file}"
  if [[ "${remove_cert_dir}" == "1" ]]; then
    rm -rf "${cert_dir}"
  fi
  exit "${exit_code}"
}
trap cleanup EXIT INT TERM

if [[ "${BUILD_IMAGE:-0}" == "1" ]]; then
  "${script_dir}/docker-build.sh" backend
  "${script_dir}/docker-build-microservice-images.sh"
fi

: "${DIPOLE_IMAGE:=dipole-server:latest}"
: "${DIPOLE_MIGRATE_IMAGE:=dipole-migrate:latest}"
: "${DIPOLE_CORE_IMAGE:=dipole-core:latest}"
: "${DIPOLE_GATEWAY_IMAGE:=dipole-gateway:latest}"
: "${DIPOLE_MESSAGE_IMAGE:=dipole-message:latest}"
: "${DIPOLE_SYNC_IMAGE:=dipole-sync:latest}"
: "${DIPOLE_SEARCH_IMAGE:=dipole-search:latest}"
: "${DIPOLE_SEARCH_INDEXER_IMAGE:=dipole-search-indexer:latest}"
export DIPOLE_IMAGE DIPOLE_MIGRATE_IMAGE DIPOLE_CORE_IMAGE DIPOLE_GATEWAY_IMAGE
export DIPOLE_MESSAGE_IMAGE DIPOLE_SYNC_IMAGE DIPOLE_SEARCH_IMAGE DIPOLE_SEARCH_INDEXER_IMAGE

INTERNAL_CERT_DIR="${cert_dir}" "${script_dir}/generate-internal-certs.sh" >/dev/null
compose config --quiet
compose up -d --wait
curl --fail --silent --show-error --connect-timeout 2 --max-time 5 "http://127.0.0.1:${gateway_port}/health" | grep -q '"component":"gateway"'

for service in core message sync gateway; do
  compose exec -T "${service}" wget -q -O - http://127.0.0.1:9100/livez | grep -qx alive
  compose exec -T "${service}" wget -q -O - http://127.0.0.1:9100/readyz | grep -qx ready
done

if [[ "${SMOKE_SEARCH_PROFILE:-0}" == "1" ]]; then
  for service in search search-indexer; do
    compose exec -T "${service}" wget -q -O - http://127.0.0.1:9100/livez | grep -qx alive
    compose exec -T "${service}" wget -q -O - http://127.0.0.1:9100/readyz | grep -qx ready
  done
fi

printf 'isolated microservices smoke passed: project=%s gateway_port=%s search_profile=%s\n' "${project}" "${gateway_port}" "${SMOKE_SEARCH_PROFILE:-0}"
