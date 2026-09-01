#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

binary="$(mktemp "${TMPDIR:-/tmp}/dipole-multipart-presigned-rollout.XXXXXX")"
trap 'rm -f "$binary"' EXIT

go build -o "$binary" ./cmd/tools/multipart-presigned-rollout-evidence
set +e
"$binary" "$@"
result=$?
set -e
exit "$result"
