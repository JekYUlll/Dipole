#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
docker run --rm --entrypoint=promtool \
  -v "${ROOT_DIR}/deploy/observability:/etc/prometheus:ro" \
  prom/prometheus:v3.5.0 check rules /etc/prometheus/agent-task-waiting-alerts.yml
docker run --rm --entrypoint=promtool \
  -v "${ROOT_DIR}/deploy/observability:/etc/prometheus:ro" \
  prom/prometheus:v3.5.0 test rules /etc/prometheus/agent-task-waiting-alerts.test.yml
