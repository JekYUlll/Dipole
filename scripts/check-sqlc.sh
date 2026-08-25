#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repo_root}"

scripts/sqlc.sh generate

if ! git diff --quiet -- internal/data/mysql/generated; then
  echo "sqlc generated files are stale; run scripts/sqlc.sh generate" >&2
  git diff -- internal/data/mysql/generated >&2
  exit 1
fi

if [[ -n "$(git ls-files --others --exclude-standard internal/data/mysql/generated)" ]]; then
  echo "sqlc generated files are untracked; add the generated output" >&2
  exit 1
fi
