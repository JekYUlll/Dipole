#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
project="dipole-multipart-reconciliation-${RANDOM}-$$"
minio_container="${project}-minio"
redis_container="${project}-redis"
minio_port="${DIPOLE_MULTIPART_RECONCILIATION_MINIO_PORT:-$((23000 + RANDOM % 1000))}"
redis_port="${DIPOLE_MULTIPART_RECONCILIATION_REDIS_PORT:-$((24000 + RANDOM % 1000))}"
restart_redis="${DIPOLE_MULTIPART_RECONCILIATION_RESTART_REDIS:-0}"
marker_dir=""
test_pid=""

if [[ -n "${DIPOLE_REMOTE_GO_ROOT:-}" ]]; then
  [[ -x "${DIPOLE_REMOTE_GO_ROOT}/bin/go" ]] || {
    echo "configured DIPOLE_REMOTE_GO_ROOT does not contain an executable Go binary" >&2
    exit 2
  }
  export PATH="${DIPOLE_REMOTE_GO_ROOT}/bin:${PATH}"
  export GOTOOLCHAIN="${GOTOOLCHAIN:-local}"
fi

cleanup() {
  local exit_code=$?
  if [[ -n "${test_pid}" ]]; then
    kill "${test_pid}" >/dev/null 2>&1 || true
  fi
  docker rm -f "${minio_container}" "${redis_container}" >/dev/null 2>&1 || true
  if [[ -n "${marker_dir}" ]]; then
    rm -rf "${marker_dir}"
  fi
  exit "${exit_code}"
}
trap cleanup EXIT INT TERM

docker run -d --name "${minio_container}" -p "127.0.0.1:${minio_port}:9000" \
  -e MINIO_ROOT_USER=dipolereconcile \
  -e MINIO_ROOT_PASSWORD=dipolereconcilepass \
  minio/minio:RELEASE.2025-04-22T22-12-26Z server /data >/dev/null
docker run -d --name "${redis_container}" -p "127.0.0.1:${redis_port}:6379" redis:7.4 >/dev/null

for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:${minio_port}/minio/health/live" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "http://127.0.0.1:${minio_port}/minio/health/live" >/dev/null
for _ in $(seq 1 60); do
  if docker exec "${redis_container}" redis-cli ping 2>/dev/null | grep -q PONG; then
    break
  fi
  sleep 1
done
docker exec "${redis_container}" redis-cli ping 2>/dev/null | grep -q PONG

if [[ "${restart_redis}" == "1" ]]; then
  marker_dir=$(mktemp -d "${TMPDIR:-/tmp}/dipole-multipart-restart.XXXXXX")
  (
    cd "${root_dir}"
    DIPOLE_TEST_MULTIPART_RECONCILIATION_MINIO_ENDPOINT="127.0.0.1:${minio_port}" \
      DIPOLE_TEST_MULTIPART_RECONCILIATION_MINIO_ACCESS_KEY=dipolereconcile \
      DIPOLE_TEST_MULTIPART_RECONCILIATION_MINIO_SECRET_KEY=dipolereconcilepass \
      DIPOLE_TEST_MULTIPART_RECONCILIATION_REDIS_ADDR="127.0.0.1:${redis_port}" \
      DIPOLE_TEST_MULTIPART_RECONCILIATION_REDIS_RESTART_READY_FILE="${marker_dir}/ready" \
      DIPOLE_TEST_MULTIPART_RECONCILIATION_REDIS_RESTART_RESUME_FILE="${marker_dir}/resume" \
      GOTOOLCHAIN=local CGO_ENABLED=0 go test ./internal/operations/storage \
        -run '^TestMultipartReconciliationWithRealMinIOAndRedis$' -count=1
  ) >"${marker_dir}/test.log" 2>&1 &
  test_pid=$!
  for _ in $(seq 1 150); do
    [[ -f "${marker_dir}/ready" ]] && break
    if ! kill -0 "${test_pid}" >/dev/null 2>&1; then
      cat "${marker_dir}/test.log"
      exit 1
    fi
    sleep 0.1
  done
  [[ -f "${marker_dir}/ready" ]] || {
    cat "${marker_dir}/test.log"
    echo "Redis restart ready marker did not appear" >&2
    exit 1
  }
  docker restart "${redis_container}" >/dev/null
  for _ in $(seq 1 60); do
    if docker exec "${redis_container}" redis-cli ping 2>/dev/null | grep -q PONG; then
      break
    fi
    sleep 1
  done
  docker exec "${redis_container}" redis-cli ping 2>/dev/null | grep -q PONG
  touch "${marker_dir}/resume"
  if ! wait "${test_pid}"; then
    cat "${marker_dir}/test.log"
    exit 1
  fi
  test_pid=""
else
  (
    cd "${root_dir}"
    DIPOLE_TEST_MULTIPART_RECONCILIATION_MINIO_ENDPOINT="127.0.0.1:${minio_port}" \
      DIPOLE_TEST_MULTIPART_RECONCILIATION_MINIO_ACCESS_KEY=dipolereconcile \
      DIPOLE_TEST_MULTIPART_RECONCILIATION_MINIO_SECRET_KEY=dipolereconcilepass \
      DIPOLE_TEST_MULTIPART_RECONCILIATION_REDIS_ADDR="127.0.0.1:${redis_port}" \
      GOTOOLCHAIN=local CGO_ENABLED=0 go test ./internal/operations/storage \
        -run '^TestMultipartReconciliationWithRealMinIOAndRedis$' -count=1
  )
fi

if [[ "${restart_redis}" == "1" ]]; then
  printf 'Multipart reconciliation restart smoke passed: Redis restart, missing metadata, MinIO cleanup, and orphan drift were verified with isolated resources.\n'
else
  printf 'Multipart reconciliation smoke passed: matched stores, missing Redis metadata, and Redis orphan drift were detected with isolated MinIO/Redis resources.\n'
fi
