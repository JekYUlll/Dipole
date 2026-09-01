#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
container="dipole-minio-multipart-${RANDOM}-$$"
port="${DIPOLE_MINIO_MULTIPART_PORT:-$((20000 + RANDOM % 1000))}"

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
  docker rm -f "$container" >/dev/null 2>&1 || true
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

docker run -d --name "$container" \
  -p "127.0.0.1:${port}:9000" \
  -e MINIO_ROOT_USER=dipolemultipart \
  -e MINIO_ROOT_PASSWORD=dipolemultipartpass \
  minio/minio:RELEASE.2025-04-22T22-12-26Z server /data >/dev/null

for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:${port}/minio/health/live" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "http://127.0.0.1:${port}/minio/health/live" >/dev/null

(
  cd "$root_dir"
  DIPOLE_TEST_MINIO_ENDPOINT="127.0.0.1:${port}" \
    DIPOLE_TEST_MINIO_ACCESS_KEY=dipolemultipart \
    DIPOLE_TEST_MINIO_SECRET_KEY=dipolemultipartpass \
    CGO_ENABLED=0 go test ./internal/platform/storage \
      -run '^TestMinIOMultipartUploadLifecycle$' -count=1
)

printf 'MinIO multipart lifecycle smoke passed: multi-part ordering, replacement, completion, content verification, and repeat abort.\n'
