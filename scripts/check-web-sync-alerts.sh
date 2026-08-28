#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

docker run --rm --entrypoint=promtool \
  -v "$root_dir/deploy/observability:/etc/prometheus:ro" \
  prom/prometheus:v3.5.0 check config /etc/prometheus/prometheus.yml

docker run --rm --entrypoint=promtool \
  -v "$root_dir/deploy/observability:/etc/prometheus:ro" \
  prom/prometheus:v3.5.0 check rules /etc/prometheus/web-sync-alerts.yml

docker run --rm --entrypoint=promtool \
  -v "$root_dir/deploy/observability:/etc/prometheus:ro" \
  prom/prometheus:v3.5.0 test rules /etc/prometheus/web-sync-alerts.test.yml
