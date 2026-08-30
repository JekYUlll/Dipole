#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
container="dipole-minio-multipart-restart-${RANDOM}-$$"
volume="${container}-data"
port="${DIPOLE_MINIO_MULTIPART_PORT:-$((21000 + RANDOM % 1000))}"
marker_dir=$(mktemp -d "${TMPDIR:-/tmp}/dipole-multipart-restart.XXXXXX")
ready_file="${marker_dir}/ready"
resume_file="${marker_dir}/resume"
worker_pid=""

cleanup() {
  local exit_code=$?
  if [[ -n "${worker_pid}" ]] && kill -0 "${worker_pid}" 2>/dev/null; then
    kill "${worker_pid}" 2>/dev/null || true
    wait "${worker_pid}" 2>/dev/null || true
  fi
  docker rm -f "${container}" >/dev/null 2>&1 || true
  docker volume rm "${volume}" >/dev/null 2>&1 || true
  rm -rf "${marker_dir}"
  exit "${exit_code}"
}
trap cleanup EXIT INT TERM

docker volume create "${volume}" >/dev/null
docker run -d --name "${container}" -p "127.0.0.1:${port}:9000" \
  -v "${volume}:/data" \
  -e MINIO_ROOT_USER=dipolerestart \
  -e MINIO_ROOT_PASSWORD=dipolerestartpass \
  minio/minio:RELEASE.2025-04-22T22-12-26Z server /data >/dev/null

for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:${port}/minio/health/live" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "http://127.0.0.1:${port}/minio/health/live" >/dev/null

(
  cd "${root_dir}"
  DIPOLE_TEST_MINIO_ENDPOINT="127.0.0.1:${port}" \
    DIPOLE_TEST_MINIO_ACCESS_KEY=dipolerestart \
    DIPOLE_TEST_MINIO_SECRET_KEY=dipolerestartpass \
    DIPOLE_MULTIPART_RESTART_READY_FILE="${ready_file}" \
    DIPOLE_MULTIPART_RESTART_RESUME_FILE="${resume_file}" \
    GOTOOLCHAIN=local go run ./cmd/tools/multipart-restart-smoke
) >"${marker_dir}/worker.log" 2>&1 &
worker_pid=$!

for _ in $(seq 1 120); do
  [[ -f "${ready_file}" ]] && break
  if ! kill -0 "${worker_pid}" 2>/dev/null; then
    cat "${marker_dir}/worker.log" >&2
    exit 1
  fi
  sleep 1
done
[[ -f "${ready_file}" ]] || { cat "${marker_dir}/worker.log" >&2; exit 1; }

docker restart "${container}" >/dev/null
for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:${port}/minio/health/live" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "http://127.0.0.1:${port}/minio/health/live" >/dev/null
touch "${resume_file}"

if ! wait "${worker_pid}"; then
  cat "${marker_dir}/worker.log" >&2
  exit 1
fi
cat "${marker_dir}/worker.log"
