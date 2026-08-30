#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

if [[ -n "${DIPOLE_PROMTOOL_BIN:-}" ]]; then
  [[ -x "${DIPOLE_PROMTOOL_BIN}" ]] || {
    echo "configured DIPOLE_PROMTOOL_BIN is not executable" >&2
    exit 2
  }
  "${DIPOLE_PROMTOOL_BIN}" check rules "$root_dir/deploy/observability/multipart-alerts.yml"
  "${DIPOLE_PROMTOOL_BIN}" test rules "$root_dir/deploy/observability/multipart-alerts.test.yml"
  exit 0
fi

docker run --rm --entrypoint=promtool \
  -v "$root_dir/deploy/observability:/etc/prometheus:ro" \
  prom/prometheus:v3.5.0 check rules /etc/prometheus/multipart-alerts.yml

docker run --rm --entrypoint=promtool \
  -v "$root_dir/deploy/observability:/etc/prometheus:ro" \
  prom/prometheus:v3.5.0 test rules /etc/prometheus/multipart-alerts.test.yml
