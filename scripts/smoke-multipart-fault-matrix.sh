#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

if [[ -n "${DIPOLE_REMOTE_GO_ROOT:-}" ]]; then
  [[ -x "${DIPOLE_REMOTE_GO_ROOT}/bin/go" ]] || {
    echo "configured DIPOLE_REMOTE_GO_ROOT does not contain an executable Go binary" >&2
    exit 2
  }
  export PATH="${DIPOLE_REMOTE_GO_ROOT}/bin:${PATH}"
fi
export GOTOOLCHAIN="${GOTOOLCHAIN:-local}"

printf '%s\n' '==> deterministic Multipart contracts'
(cd "${root_dir}" && CGO_ENABLED=0 go test ./cmd/tools/multipart-cleanup ./internal/operations/storage ./internal/gateway/http ./internal/services/gateway/server ./internal/services/gateway/bootstrap ./internal/config -count=1)

printf '%s\n' '==> Prometheus alert rules and firing timelines'
(cd "${root_dir}" && scripts/check-multipart-alerts.sh)

printf '%s\n' '==> real MinIO/Redis reconciliation matrix'
(cd "${root_dir}" && DIPOLE_MULTIPART_RECONCILIATION_RESTART_REDIS=0 scripts/smoke-multipart-reconciliation.sh)
(cd "${root_dir}" && DIPOLE_MULTIPART_RECONCILIATION_RESTART_REDIS=1 scripts/smoke-multipart-reconciliation.sh)

printf '%s\n' 'Multipart fault matrix passed: deterministic contracts, Prometheus alerts, MinIO/Redis drift, Redis restart, cleanup race, proxy timeout, and Gateway rate limiting are covered.'
