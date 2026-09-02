#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repo_root}"

generated_dir="internal/platform/mysql/generated"
before_snapshot="$(mktemp)"
after_snapshot="$(mktemp)"
trap 'rm -f "${before_snapshot}" "${after_snapshot}"' EXIT

reject_legacy_orm() {
  local legacy_module_pattern='gorm\.io/|github\.com/jinzhu/gorm'
  local legacy_go_pattern='gorm\.io/gorm|github\.com/jinzhu/gorm|\bgorm\.[[:alpha:]_][[:alnum:]_]*|\.AutoMigrate[[:space:]]*\('

  if rg --quiet --ignore-case "${legacy_module_pattern}" go.mod go.sum; then
    echo "GORM module dependencies remain after the SQLC migration" >&2
    exit 1
  fi

  # Scan tests too: test-only GORM adapters would leave a second data-access
  # model that cannot be shared safely by the future polyglot services.
  if rg --quiet --glob '*.go' --glob '!vendor/**' --ignore-case "${legacy_go_pattern}" .; then
    echo "Go code must use database/sql and SQLC; GORM or AutoMigrate references remain" >&2
    exit 1
  fi
}

snapshot_generated() {
  find "${generated_dir}" -type f -print0 | sort -z | xargs -0 sha256sum
}

reject_legacy_orm
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
