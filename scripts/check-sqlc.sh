#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repo_root}"

generated_dir="internal/platform/mysql/generated"
before_snapshot="$(mktemp)"
after_snapshot="$(mktemp)"
trap 'rm -f "${before_snapshot}" "${after_snapshot}"' EXIT

snapshot_generated() {
  find "${generated_dir}" -type f -print0 | sort -z | xargs -0 sha256sum
}

snapshot_generated >"${before_snapshot}"
scripts/sqlc.sh generate
snapshot_generated >"${after_snapshot}"

if ! diff -u "${before_snapshot}" "${after_snapshot}"; then
  echo "sqlc generated files were stale before this check; keep the regenerated output" >&2
  exit 1
fi

if [[ -n "$(git ls-files --others --exclude-standard "${generated_dir}")" ]]; then
  echo "sqlc generated files are untracked; add the generated output" >&2
  exit 1
fi
