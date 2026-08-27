#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
docker_config=${DIPOLE_SMOKE_DOCKER_CONFIG:-/tmp/dipole-docker-anonymous}
mkdir -p "$docker_config"

DOCKER_CONFIG="$docker_config" docker run --rm \
  -v "$root_dir/deploy/observability/otel-collector.yml:/etc/otelcol-contrib/config.yml:ro" \
  ghcr.io/open-telemetry/opentelemetry-collector-releases/opentelemetry-collector-contrib:0.159.0 \
  validate --config=/etc/otelcol-contrib/config.yml

docker run --rm --entrypoint=promtool \
  -v "$root_dir/deploy/observability:/etc/prometheus:ro" \
  prom/prometheus:v3.5.0 check rules /etc/prometheus/agent-otel-alerts.yml

docker run --rm --entrypoint=promtool \
  -v "$root_dir/deploy/observability:/etc/prometheus:ro" \
  prom/prometheus:v3.5.0 test rules /etc/prometheus/agent-otel-alerts.test.yml
