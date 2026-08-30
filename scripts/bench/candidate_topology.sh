#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
COMPOSE_FILE="${C1_COMPOSE_FILE:-deploy/compose/docker-compose.dist.yml}"
C1_PROJECT="${C1_PROJECT:-dipole-c1}"
C1_CONTAINER_PREFIX="${C1_CONTAINER_PREFIX:-dipole-c1}"
C1_READY_TIMEOUT_SECONDS="${C1_READY_TIMEOUT_SECONDS:-180}"
C1_ENABLE_OPTIONAL_SERVICES="${C1_ENABLE_OPTIONAL_SERVICES:-0}"

export DIPOLE_CONTAINER_PREFIX="${C1_CONTAINER_PREFIX}"
export DIPOLE_MYSQL_PORT="${C1_MYSQL_PORT:-13306}"
export DIPOLE_REDIS_PORT="${C1_REDIS_PORT:-16379}"
export DIPOLE_KAFKA_EXTERNAL_PORT="${C1_KAFKA_EXTERNAL_PORT:-19094}"
export DIPOLE_KAFDROP_PORT="${C1_KAFDROP_PORT:-19099}"
export DIPOLE_MINIO_PORT="${C1_MINIO_PORT:-19000}"
export DIPOLE_MINIO_CONSOLE_PORT="${C1_MINIO_CONSOLE_PORT:-19001}"
export DIPOLE_NODE1_PORT="${C1_NODE1_PORT:-18081}"
export DIPOLE_NODE2_PORT="${C1_NODE2_PORT:-18082}"
export DIPOLE_NODE3_PORT="${C1_NODE3_PORT:-18083}"
export DIPOLE_HTTP_PORT="${C1_HTTP_PORT:-18080}"
export DIPOLE_HTTPS_PORT="${C1_HTTPS_PORT:-18443}"
export DIPOLE_NETWORK_SUBNET="${C1_NETWORK_SUBNET:-10.201.0.0/24}"
export DIPOLE_AI_RUNTIME_MODE=off

usage() {
  echo "Usage: $0 up <image>|status|down"
  echo ""
  echo "up      Verify a clean same-revision image, pin its image ID, and start the isolated topology"
  echo "        Set C1_ENABLE_OPTIONAL_SERVICES=1 to include Kafdrop and Nginx"
  echo "status  Show the isolated topology without changing it"
  echo "down    Stop the isolated topology while preserving its named volumes"
}

compose() {
  docker compose \
    --project-name "${C1_PROJECT}" \
    -f "${ROOT_DIR}/${COMPOSE_FILE}" \
    "$@"
}

prepare_certs() {
  local cert_dir="${ROOT_DIR}/deploy/compose/certs/local"
  local cert_file="${cert_dir}/dipole-local.pem"
  local key_file="${cert_dir}/dipole-local-key.pem"

  mkdir -p "${cert_dir}"
  if [[ ! -s "${cert_file}" || ! -s "${key_file}" ]]; then
    umask 077
    openssl req -x509 -nodes -newkey rsa:2048 \
      -keyout "${key_file}" \
      -out "${cert_file}" \
      -days "${C1_CERT_VALID_DAYS:-7}" \
      -subj "/CN=dipole-local" \
      -addext "subjectAltName=DNS:localhost,DNS:dipole.local,IP:127.0.0.1"
    chmod 600 "${key_file}"
    chmod 644 "${cert_file}"
  fi
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "required command not found: $1" >&2
    exit 1
  }
}

verify_candidate_image() {
  local image="$1"
  local expected_revision image_id revision source_dirty

  if [[ -z "${image}" ]]; then
    echo "candidate image is required" >&2
    exit 1
  fi
  if [[ -n "$(git -C "${ROOT_DIR}" status --porcelain --untracked-files=no)" ]]; then
    echo "candidate source tree has changes; commit them before deployment" >&2
    exit 1
  fi

  expected_revision="$(git -C "${ROOT_DIR}" rev-parse HEAD)"
  image_id="$(docker image inspect --format '{{.Id}}' "${image}")"
  revision="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "${image_id}")"
  source_dirty="$(docker image inspect --format '{{index .Config.Labels "io.dipole.source.dirty"}}' "${image_id}")"
  if [[ "${revision}" != "${expected_revision}" ]]; then
    echo "candidate image revision ${revision} does not match source ${expected_revision}" >&2
    exit 1
  fi
  if [[ "${source_dirty}" != "false" ]]; then
    echo "candidate image must carry io.dipole.source.dirty=false" >&2
    exit 1
  fi

  DIPOLE_IMAGE="${image_id}"
  export DIPOLE_IMAGE
  echo "candidate image verified: ${DIPOLE_IMAGE} revision=${revision}"
}

wait_ready() {
  local deadline=$((SECONDS + C1_READY_TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    if curl --fail --silent "http://127.0.0.1:${DIPOLE_NODE1_PORT}/health" >/dev/null \
      && curl --fail --silent "http://127.0.0.1:${DIPOLE_NODE2_PORT}/health" >/dev/null \
      && curl --fail --silent "http://127.0.0.1:${DIPOLE_NODE3_PORT}/health" >/dev/null; then
      return 0
    fi
    sleep 2
  done
  echo "candidate topology did not become ready within ${C1_READY_TIMEOUT_SECONDS}s" >&2
  compose ps >&2
  return 1
}

for command in docker git; do
  require_command "${command}"
done

case "${1:-}" in
  up)
    require_command curl
    require_command openssl
    verify_candidate_image "${2:-${C1_IMAGE:-}}"
    prepare_certs
    compose up -d --wait --wait-timeout "${C1_READY_TIMEOUT_SECONDS}" mysql redis kafka minio
    compose up --no-deps minio-init
    compose run --rm --no-deps --entrypoint /app/dipole-migrate dipole-node1 -direction up
    compose up -d dipole-node1 dipole-node2 dipole-node3
    if [[ "${C1_ENABLE_OPTIONAL_SERVICES}" == "1" ]]; then
      compose up -d kafdrop nginx
    fi
    wait_ready
    compose ps
    ;;
  status)
    compose ps
    ;;
  down)
    compose down --remove-orphans
    ;;
  *)
    usage
    exit 1
    ;;
esac
