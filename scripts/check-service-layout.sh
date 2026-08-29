#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
expected_services=(core gateway message sync search search-indexer)
if [[ ! -f "${root_dir}/cmd/services/README.md" ]]; then
  echo "service entrypoint index is missing: cmd/services/README.md" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/docs/architecture/SERVICE-BOUNDARIES.md" ]]; then
  echo "service boundary manifest is missing: docs/architecture/SERVICE-BOUNDARIES.md" >&2
  exit 1
fi
if ! git -C "${root_dir}" ls-files --error-unmatch docs/architecture/SERVICE-BOUNDARIES.md >/dev/null 2>&1; then
  echo "service boundary manifest is not tracked: docs/architecture/SERVICE-BOUNDARIES.md" >&2
  exit 1
fi
if [[ ! -f "${root_dir}/internal/services/search/application/search.go" ]]; then
  echo "Search application implementation is outside its service boundary" >&2
  exit 1
fi
if [[ -e "${root_dir}/internal/app/search.go" || -e "${root_dir}/internal/app/search_test.go" ]]; then
  echo "legacy shared Search application path remains under internal/app" >&2
  exit 1
fi
for service in "${expected_services[@]}"; do
  if [[ ! -f "${root_dir}/cmd/services/${service}/main.go" ]]; then
    echo "missing service entrypoint: cmd/services/${service}/main.go" >&2
    exit 1
  fi
done

for legacy in server gateway message-service sync-service search-service search-indexer; do
  if [[ -e "${root_dir}/cmd/${legacy}" ]]; then
    echo "legacy service entrypoint remains at cmd/${legacy}" >&2
    exit 1
  fi
done

if [[ -n "$(find "${root_dir}/cmd" -mindepth 1 -maxdepth 1 -type d ! -name services ! -name tools -print -quit)" ]]; then
  echo "unclassified command directory remains directly under cmd/" >&2
  exit 1
fi

echo "service command layout: ok"
