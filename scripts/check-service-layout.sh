#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
expected_services=(core gateway message sync search search-indexer)
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

echo "service command layout: ok"
