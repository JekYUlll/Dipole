#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
config="${root_dir}/deploy/observability/alertmanager.yml"
image="${DIPOLE_ALERTMANAGER_IMAGE:-quay.io/prometheus/alertmanager:v0.28.1}"

if [[ -n "${DIPOLE_AMTOOL_BIN:-}" ]]; then
  "${DIPOLE_AMTOOL_BIN}" check-config "${config}"
  exit 0
fi

docker run --rm --entrypoint=amtool \
  -v "${config}:/etc/alertmanager/alertmanager.yml:ro" \
  "${image}" check-config /etc/alertmanager/alertmanager.yml
