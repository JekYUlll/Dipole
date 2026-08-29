#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
project_name="dipole-agent-otel-${RANDOM}"
docker_config=${DIPOLE_SMOKE_DOCKER_CONFIG:-/tmp/dipole-docker-anonymous}
mkdir -p "$docker_config"

compose=(docker compose -p "$project_name" -f "$root_dir/deploy/compose/docker-compose.microservices.yml" --profile observability)
cleanup() {
  DOCKER_CONFIG="$docker_config" DIPOLE_INTERNAL_RPC_SHARED_SECRET=otel-smoke-only \
    "${compose[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

DOCKER_CONFIG="$docker_config" DIPOLE_INTERNAL_RPC_SHARED_SECRET=otel-smoke-only \
  "${compose[@]}" up -d tempo otel-collector

for _ in $(seq 1 60); do
  if curl -fsS http://127.0.0.1:13133/ >/dev/null 2>&1 && curl -fsS http://127.0.0.1:3200/ready >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS http://127.0.0.1:13133/ >/dev/null
curl -fsS http://127.0.0.1:3200/ready >/dev/null

npm --prefix "$root_dir/services/agent-runtime" run build >/dev/null
trace_id=$(
  cd "$root_dir"
  DIPOLE_AGENT_OTEL_ENABLED=true \
  OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://127.0.0.1:4318/v1/traces \
  OTEL_TRACES_SAMPLER_ARG=1 \
  OTEL_SERVICE_NAME=dipole-agent-smoke \
  node --input-type=module <<'NODE'
import { createAgentObservabilityRuntime, loadAgentObservabilityConfig } from "./services/agent-runtime/dist/observability/agent-observability-runtime.js";
import { AgentTelemetry } from "./services/agent-runtime/dist/observability/agent-telemetry.js";

const runtime = createAgentObservabilityRuntime(loadAgentObservabilityConfig(process.env));
runtime.start();
const traceId = await new AgentTelemetry().withSpan("agent.otel.smoke", { taskId: "TASK-SMOKE" }, async span => {
  span.setAttribute("dipole.agent.smoke", true);
  return span.spanContext().traceId;
});
await runtime.stop();
process.stdout.write(traceId);
NODE
)

if [[ ! "$trace_id" =~ ^[0-9a-f]{32}$ ]]; then
  echo "Agent OTel smoke produced an invalid trace ID" >&2
  exit 1
fi

trace_json=""
for _ in $(seq 1 30); do
  trace_json=$(curl -fsS -H 'Accept: application/json' "http://127.0.0.1:3200/api/traces/$trace_id" 2>/dev/null || true)
  if [[ "$trace_json" == *"agent.otel.smoke"* && "$trace_json" == *"dipole-agent-smoke"* ]]; then
    break
  fi
  sleep 1
done
if [[ "$trace_json" != *"agent.otel.smoke"* || "$trace_json" != *"dipole-agent-smoke"* ]]; then
  echo "Agent OTel trace was not queryable from Tempo" >&2
  exit 1
fi

metrics=$(curl -fsS http://127.0.0.1:8888/metrics)
if [[ "$metrics" != *"otelcol_receiver_accepted_spans"* || "$metrics" != *"otelcol_exporter_sent_spans"* ]]; then
  echo "Collector accepted/sent span metrics are missing" >&2
  exit 1
fi

echo "Agent OTel smoke passed: trace $trace_id traversed Collector and is queryable from Tempo."
