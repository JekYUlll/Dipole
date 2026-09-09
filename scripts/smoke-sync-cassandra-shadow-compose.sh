#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root_dir=$(cd "${script_dir}/.." && pwd)
project="${COMPOSE_PROJECT_NAME:-dipole-sync-cassandra-shadow-${RANDOM}-$$}"
compose_file="${root_dir}/deploy/compose/docker-compose.microservices.yml"
shadow_file="${root_dir}/deploy/microservices/cassandra-shadow.yml"
sync_shadow_file="${root_dir}/deploy/microservices/cassandra-sync-shadow.yml"
revision=$(git -C "${root_dir}" rev-parse --short=12 HEAD)
cert_dir="${DIPOLE_INTERNAL_CERT_DIR:-$(mktemp -d -t dipole-sync-cassandra-shadow-certs.XXXXXX)}"
remove_cert_dir=0
if [[ -z "${DIPOLE_INTERNAL_CERT_DIR:-}" ]]; then
  remove_cert_dir=1
fi

: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=$(openssl rand -hex 32)}"
: "${DIPOLE_AGENT_MODEL_PROVIDER_NAME:=sync-cassandra-shadow-smoke}"
: "${DIPOLE_AGENT_MODEL_BASE_URL:=https://models.invalid/v1}"
: "${DIPOLE_AGENT_MODEL_API_KEY:=sync-cassandra-shadow-smoke-no-network}"
: "${DIPOLE_AGENT_MODEL_ROUTES:=sync-cassandra-shadow-smoke/deterministic}"
: "${DIPOLE_AGENT_MODEL_CONTEXT_PROFILES:=[{\"route\":\"sync-cassandra-shadow-smoke/deterministic\",\"contextWindowTokens\":32768,\"utf8BytesPerToken\":3,\"safetyMarginBps\":1500}]}"
export DIPOLE_INTERNAL_RPC_SHARED_SECRET DIPOLE_AGENT_MODEL_PROVIDER_NAME DIPOLE_AGENT_MODEL_BASE_URL
export DIPOLE_AGENT_MODEL_API_KEY DIPOLE_AGENT_MODEL_ROUTES DIPOLE_AGENT_MODEL_CONTEXT_PROFILES
export DIPOLE_INTERNAL_CERT_DIR="${cert_dir}"
: "${DIPOLE_SYNC_IMAGE:=dipole-sync:cassandra-shadow-${revision}}"
: "${DIPOLE_CASSANDRA_PROJECTOR_IMAGE:=dipole-cassandra-projector:cassandra-shadow-${revision}}"
export DIPOLE_SYNC_IMAGE
export DIPOLE_CASSANDRA_PROJECTOR_IMAGE

build_service_image() {
  local binary=$1
  local package=$2
  local image=$3
  (
    build_context=$(mktemp -d -t dipole-sync-cassandra-shadow-image.XXXXXX)
    trap 'rm -rf "${build_context}"' EXIT

    GOFLAGS=-mod=mod CGO_ENABLED=0 go build -o "${build_context}/${binary}" "${package}"
    docker build \
      --file "${root_dir}/deploy/images/go-service.Dockerfile" \
      --tag "${image}" \
      --build-arg "DIPOLE_BINARY=${binary}" \
      --build-arg "DIPOLE_VCS_REVISION=$(git -C "${root_dir}" rev-parse HEAD)" \
      --build-arg "DIPOLE_BUILD_CREATED=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      --build-arg DIPOLE_BUILD_DIRTY=false \
      "${build_context}"
  )
}

compose() {
  docker compose -p "${project}" --profile cassandra-shadow \
    -f "${compose_file}" -f "${shadow_file}" -f "${sync_shadow_file}" "$@"
}

cleanup() {
  local exit_code=$?
  if [[ "${KEEP_STACK:-0}" == "1" && "${exit_code}" != "0" ]]; then
    printf 'Sync Cassandra shadow Compose stack retained: project=%s\n' "${project}" >&2
  else
    compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  fi
  if [[ "${remove_cert_dir}" == "1" ]]; then
    rm -rf "${cert_dir}"
  fi
  exit "${exit_code}"
}
trap cleanup EXIT INT TERM

INTERNAL_CERT_DIR="${cert_dir}" "${script_dir}/generate-internal-certs.sh" >/dev/null
if [[ "${DIPOLE_SYNC_SMOKE_BUILD_IMAGE:-1}" == "1" ]]; then
  build_service_image dipole-sync ./cmd/services/sync "${DIPOLE_SYNC_IMAGE}"
  build_service_image dipole-cassandra-projector ./cmd/tools/cassandra-projector "${DIPOLE_CASSANDRA_PROJECTOR_IMAGE}"
fi
compose config --quiet
compose up -d --wait --wait-timeout "${CASSANDRA_SHADOW_READY_TIMEOUT_SECONDS:-240}" cassandra-init cassandra-projector sync

test "$(compose exec -T sync wget -q -O - http://127.0.0.1:9100/readyz)" = ready
sync_env=$(compose exec -T sync sh -ec 'printf "%s:%s:%s\n" "$DIPOLE_CASSANDRA_ENABLED" "$DIPOLE_SYNC_CASSANDRA_SHADOW_HYDRATION" "$DIPOLE_SYNC_CASSANDRA_PRIMARY_HYDRATION"')
test "${sync_env}" = "true:true:false"

printf 'Sync Cassandra shadow Compose smoke passed: project=%s shadow=true primary=false schema-init=true ready=true\n' "${project}"
