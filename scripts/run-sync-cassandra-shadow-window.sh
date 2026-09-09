#!/usr/bin/env bash
set -euo pipefail

# Opens a bounded public Sync hydration shadow window. Client responses remain
# MySQL-backed; Cassandra is read only for asynchronous comparison.
root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
project="${DIPOLE_SYNC_SHADOW_WINDOW_PROJECT:-dipole-experience}"
env_file="${DIPOLE_ENV_FILE:-${root_dir}/.env}"
cert_dir="${DIPOLE_INTERNAL_CERT_DIR:-}"
window_seconds="${DIPOLE_SYNC_SHADOW_WINDOW_SECONDS:-120}"
exercise="${DIPOLE_SYNC_SHADOW_WINDOW_EXERCISE:-}"
output_dir="${DIPOLE_SYNC_SHADOW_WINDOW_OUTPUT_DIR:-}"
confirm="${DIPOLE_SYNC_SHADOW_WINDOW_CONFIRM:-}"
revision=$(git -C "${root_dir}" rev-parse HEAD)
image="${DIPOLE_SYNC_IMAGE:-dipole-sync:sync-shadow-${revision:0:12}}"

if [[ "${confirm}" != "yes" ]]; then
  echo "Set DIPOLE_SYNC_SHADOW_WINDOW_CONFIRM=yes to open a public shadow window" >&2
  exit 2
fi
if [[ ! -r "${env_file}" ]]; then
  echo "DIPOLE_ENV_FILE must name a readable Compose env file" >&2
  exit 2
fi
if [[ "${cert_dir}" != /* || ! -f "${cert_dir}/ca.pem" || ! -f "${cert_dir}/sync.pem" || ! -f "${cert_dir}/sync-key.pem" ]]; then
  echo "DIPOLE_INTERNAL_CERT_DIR must name an absolute readable internal certificate directory" >&2
  exit 2
fi
if ! [[ "${window_seconds}" =~ ^[0-9]+$ ]] || (( window_seconds < 30 || window_seconds > 900 )); then
  echo "DIPOLE_SYNC_SHADOW_WINDOW_SECONDS must be between 30 and 900" >&2
  exit 2
fi
if [[ ! -x "${exercise}" || "${exercise}" != /* ]]; then
  echo "DIPOLE_SYNC_SHADOW_WINDOW_EXERCISE must name an executable absolute path" >&2
  exit 2
fi
if [[ -z "${output_dir}" || "${output_dir}" != /* || -e "${output_dir}" ]]; then
  echo "DIPOLE_SYNC_SHADOW_WINDOW_OUTPUT_DIR must name a new absolute path" >&2
  exit 2
fi

mkdir -p "${output_dir}"
compose_files=(
  "${root_dir}/deploy/compose/docker-compose.microservices.yml"
  "${root_dir}/deploy/microservices/remote-gpu-mysql-aio-compat.yml"
  "${root_dir}/deploy/microservices/agent-experience.yml"
  "${root_dir}/deploy/microservices/cassandra-shadow.yml"
)
shadow_file="${root_dir}/deploy/microservices/cassandra-sync-shadow.yml"
compose_base=(docker compose --env-file "${env_file}" -p "${project}" --profile cassandra-shadow)
for file in "${compose_files[@]}"; do
  compose_base+=(-f "${file}")
done

compose_base_cmd() { "${compose_base[@]}" "$@"; }
compose_shadow_cmd() { "${compose_base[@]}" -f "${shadow_file}" "$@"; }

wait_for_sync_ready() {
  local command_name=$1
  for _ in $(seq 1 30); do
    if "${command_name}" exec -T sync wget -q -O - http://127.0.0.1:9100/readyz 2>/dev/null | grep -qx ready; then
      return 0
    fi
    sleep 1
  done
  return 1
}

build_sync_image() {
  (
    context=$(mktemp -d -t dipole-sync-shadow-window.XXXXXX)
    trap 'rm -rf "${context}"' EXIT
    GOFLAGS=-mod=mod CGO_ENABLED=0 go build -o "${context}/dipole-sync" ./cmd/services/sync
    docker build --file "${root_dir}/deploy/images/go-service.Dockerfile" --tag "${image}" \
      --build-arg DIPOLE_BINARY=dipole-sync \
      --build-arg "DIPOLE_VCS_REVISION=${revision}" \
      --build-arg "DIPOLE_BUILD_CREATED=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      --build-arg DIPOLE_BUILD_DIRTY=false "${context}"
  )
}

restore_mysql_hydration() {
  compose_base_cmd up -d --no-deps sync >/dev/null 2>&1 || true
  wait_for_sync_ready compose_base_cmd || true
}
trap restore_mysql_hydration EXIT INT TERM

build_sync_image
export DIPOLE_SYNC_IMAGE="${image}"
compose_shadow_cmd config --quiet
compose_shadow_cmd up -d --no-deps sync
wait_for_sync_ready compose_shadow_cmd
compose_shadow_cmd exec -T sync wget -q -O - http://127.0.0.1:9100/metrics >"${output_dir}/metrics-start.prom"
date -u +%Y-%m-%dT%H:%M:%SZ >"${output_dir}/window-start.txt"

"${exercise}"
sleep "${window_seconds}"

compose_shadow_cmd exec -T sync wget -q -O - http://127.0.0.1:9100/metrics >"${output_dir}/metrics-end.prom"
date -u +%Y-%m-%dT%H:%M:%SZ >"${output_dir}/window-end.txt"
GOFLAGS=-mod=mod CGO_ENABLED=0 go run ./cmd/tools/sync-cassandra-hydration-snapshot \
  -metrics-start "${output_dir}/metrics-start.prom" \
  -metrics-end "${output_dir}/metrics-end.prom" \
  -service dipole-sync -revision "${revision}" -mode shadow \
  -window-start "$(cat "${output_dir}/window-start.txt")" \
  -window-end "$(cat "${output_dir}/window-end.txt")" >"${output_dir}/evidence.json"

restore_mysql_hydration
trap - EXIT INT TERM
printf 'Sync Cassandra shadow window completed: output=%s revision=%s\n' "${output_dir}" "${revision}"
