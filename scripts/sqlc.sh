#!/usr/bin/env bash
set -euo pipefail

required_version="v1.31.1"
sqlc_bin="${SQLC_BIN:-sqlc}"

if ! command -v "${sqlc_bin}" >/dev/null 2>&1; then
  echo "sqlc ${required_version} is required; install it with:" >&2
  echo "  go install github.com/sqlc-dev/sqlc/cmd/sqlc@${required_version}" >&2
  exit 1
fi

actual_version="$(${sqlc_bin} version)"
if [[ "${actual_version}" != "${required_version}" ]]; then
  echo "sqlc version mismatch: expected ${required_version}, got ${actual_version}" >&2
  exit 1
fi

exec "${sqlc_bin}" "$@"
