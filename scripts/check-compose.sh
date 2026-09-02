#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

# Keep component-level cluster smoke separate from the business cluster
# composition, while validating the latter as an explicit override.
business_topology_doc="docs/architecture/BUSINESS-TOPOLOGY.md"
[[ -f "$business_topology_doc" ]] || {
  echo "business topology contract is missing: ${business_topology_doc}" >&2
  exit 1
}
grep -F "当前仓库已具备可渲染的 MySQL Router/InnoDB Cluster、Kafka 三节点、Redis Sentinel 业务组合拓扑，微服务默认路径仍是单节点。" \
  "$business_topology_doc" >/dev/null || {
  echo "business topology contract must remain fail-closed" >&2
  exit 1
}

: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=static-compose-validation-only}"
export DIPOLE_INTERNAL_RPC_SHARED_SECRET

for file in docker-compose.yml deploy/compose/docker-compose*.yml; do
  [[ "$file" == "deploy/compose/docker-compose.business-cluster.yml" ]] && continue
  docker compose -f "$file" config --quiet
done

check_bind_sources() {
  local file="$1" source
  while IFS= read -r source; do
    [[ -z "$source" ]] && continue
    case "$source" in
      "$ROOT_DIR"/*) ;;
      *)
        echo "Compose bind source escapes repository root: ${file} -> ${source}" >&2
        return 1
        ;;
    esac
  done < <(docker compose -f "$file" config --format json | jq -r '.. | objects | select(.type? == "bind") | .source? // empty')
}

for file in docker-compose.yml deploy/compose/docker-compose*.yml; do
  [[ "$file" == "deploy/compose/docker-compose.business-cluster.yml" ]] && continue
  check_bind_sources "$file"
done

business_cluster_config="$({
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
    docker compose \
      -f deploy/compose/docker-compose.microservices.yml \
      -f deploy/compose/docker-compose.business-cluster.yml config --format json
})"
jq -e '
  .services.kafka.environment.KAFKA_DEFAULT_REPLICATION_FACTOR == "3"
  and .services.kafka.environment.KAFKA_MIN_INSYNC_REPLICAS == "2"
  and .services["kafka-2"].environment.KAFKA_NODE_ID == "2"
  and .services["kafka-3"].environment.KAFKA_NODE_ID == "3"
  and .services.core.environment.DIPOLE_KAFKA_BROKERS == "kafka:9092,kafka-2:9092,kafka-3:9092"
  and .services.message.environment.DIPOLE_KAFKA_REQUIRED_ACKS == "all"
  and .services.gateway.environment.DIPOLE_REDIS_MODE == "sentinel"
  and .services.gateway.environment.DIPOLE_REDIS_SENTINEL_MASTER_NAME == "dipole-master"
  and .services.mysql.image == "container-registry.oracle.com/mysql/community-router:8.4"
  and .services.mysql.environment.MYSQL_HOST == "mysql-1"
  and any(.services.mysql.healthcheck.test[]?; . == "3306")
  and .services["mysql-cluster-init"].depends_on["mysql-1"].condition == "service_healthy"
  and .services["mysql-cluster-init"].depends_on["mysql-2"].condition == "service_healthy"
  and .services["mysql-cluster-init"].depends_on["mysql-3"].condition == "service_healthy"
  and .services["mysql-1"].image == "mysql:8.4.8"
  and .services["mysql-2"].image == "mysql:8.4.8"
  and .services["mysql-3"].image == "mysql:8.4.8"
  and any(.services["sentinel-3"].volumes[]?;
    (.source? | (type == "string" and endswith("/deploy/redis/business-sentinel.conf")))
  )
' <<<"${business_cluster_config}" >/dev/null

cluster_config="$(docker compose --profile observability -f deploy/compose/docker-compose.cluster.yml config --format json)"
jq -e '
  any(.services.prometheus.volumes[];
    (.source | endswith("/deploy/observability/duplicate-hydration-alerts.yml"))
    and .target == "/etc/prometheus/duplicate-hydration-alerts.yml"
  )
  and any(.services.prometheus.volumes[];
    (.source | endswith("/deploy/observability/agent-timeline-repair-alerts.yml"))
    and .target == "/etc/prometheus/agent-timeline-repair-alerts.yml"
  )
' <<<"${cluster_config}" >/dev/null

default_microservices_config="$(docker compose -f deploy/compose/docker-compose.microservices.yml config --format json)"
if ! awk '/^  agent:/{inside=1; next} inside && /^  [^[:space:]]/{exit} inside && /path: \.\.\/\.\.\/\.env/{found=1} END{exit found ? 0 : 1}' deploy/compose/docker-compose.microservices.yml; then
  echo "Agent service must load the optional root .env file" >&2
  exit 1
fi
jq -e '
  (.services["realtime-cpp"] == null)
  and .services.gateway.environment.DIPOLE_REALTIME_DELIVERY == "go"
  and .services.gateway.environment.DIPOLE_INTERNAL_RPC_DELIVERY_PRIMARY_ENABLED == "false"
  and .services.migrate.image == "dipole-migrate:latest"
  and .services.migrate.entrypoint == ["/app/service"]
  and .services.core.image == "dipole-core:latest"
  and .services.core.entrypoint == ["/app/service"]
  and .services.core.environment.DIPOLE_CORE_MESSAGE_TRANSPORT == "grpc"
  and .services.core.environment.DIPOLE_MESSAGE_TRANSPORT == "grpc"
  and .services.core.environment.DIPOLE_AI_ENABLED == "true"
  and .services.core.environment.DIPOLE_AI_RUNTIME_MODE == "remote"
  and .services.core.environment.DIPOLE_INTERNAL_RPC_AGENT_CONVERSATION_SEARCH_ENABLED == "false"
  and .services.gateway.image == "dipole-gateway:latest"
  and .services.gateway.entrypoint == ["/app/service"]
  and .services.gateway.environment.DIPOLE_GATEWAY_AGENT_DEFINITION_ENABLED == "false"
  and .services.gateway.environment.DIPOLE_GATEWAY_AGENT_SUBSCRIPTION_ENABLED == "false"
  and .services.message.image == "dipole-message:latest"
  and .services.message.entrypoint == ["/app/service"]
  and .services.sync.image == "dipole-sync:latest"
  and .services.sync.entrypoint == ["/app/service"]
  and .services.agent.image == "dipole-agent:latest"
  and (.services.agent.build.context | endswith("/services/agent-runtime"))
  and .services.agent.environment.DIPOLE_AGENT_KAFKA_ENABLED == "true"
  and .services.agent.environment.DIPOLE_AGENT_RUNTIME_MODE == "shadow"
  and .services.agent.environment.DIPOLE_AGENT_UUID == "UAI000000000000000001"
  and .services.agent.environment.DIPOLE_AGENT_RETRIEVAL_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED == "false"
  and .services.agent.depends_on.core.condition == "service_healthy"
  and ((.services.core.depends_on // {}) | has("message") | not)
  and ((.services.message.depends_on // {}) | has("core") | not)
  and .services.gateway.depends_on.sync.condition == "service_healthy"
' <<<"${default_microservices_config}" >/dev/null

repair_profile_config="$(
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
    docker compose --profile agent-timeline-repair \
      -f deploy/compose/docker-compose.microservices.yml config --format json
)"
jq -e '
  .services["agent-timeline-repair"].image == "dipole-agent-timeline-repair:latest"
  and .services["agent-timeline-repair"].entrypoint == ["/app/service"]
' <<<"${repair_profile_config}" >/dev/null

active_agent_config="$({
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
  DIPOLE_AGENT_RELEASE_MANIFEST_FILE=/tmp/dipole-agent-release-manifest-check.json \
  DIPOLE_AGENT_CANDIDATE_VERSION=agent-runtime@compose-check \
  DIPOLE_AGENT_CONTROL_ENABLED=true \
  DIPOLE_AGENT_MCP_SERVER_ENABLED=true \
  DIPOLE_AGENT_ACTIVE_KAFKA_GROUP_ID=dipole-agent-active-compose-check \
  DIPOLE_AGENT_MODEL_PROVIDER_NAME=openai \
  DIPOLE_AGENT_MODEL_BASE_URL=https://models.example.test/v1 \
  DIPOLE_AGENT_MODEL_API_KEY=compose-check-model-key \
  DIPOLE_AGENT_MODEL_ROUTES=openai/gpt-5-mini \
  DIPOLE_AGENT_CONTEXT_COMPILER_VERSION=v2 \
  DIPOLE_AGENT_MODEL_CONTEXT_PROFILES='[{"route":"openai/gpt-5-mini","contextWindowTokens":32768,"utf8BytesPerToken":3,"safetyMarginBps":1500}]' \
  DIPOLE_AGENT_TEMPORAL_ADDRESS=temporal:7233 \
  DIPOLE_AGENT_TEMPORAL_NAMESPACE=dipole \
  DIPOLE_AGENT_TEMPORAL_TASK_QUEUE=dipole-agent-active-compose-check \
    docker compose -f deploy/compose/docker-compose.microservices.yml \
      -f deploy/microservices/agent-active.yml config --format json
})"
jq -e '
  .services.agent.environment.DIPOLE_AGENT_RUNTIME_MODE == "remote"
  and .services.agent.environment.DIPOLE_AGENT_CANDIDATE_VERSION == "agent-runtime@compose-check"
  and .services.agent.environment.DIPOLE_AGENT_RELEASE_MANIFEST == "/run/dipole/release/manifest.json"
  and .services.agent.environment.DIPOLE_AGENT_KAFKA_GROUP_ID == "dipole-agent-active-compose-check"
  and (.services.agent.environment.DIPOLE_AGENT_KAFKA_GROUP_ID | startswith("dipole-agent-active-"))
  and .services.agent.environment.DIPOLE_AGENT_TRIGGER_MODE == "direct_target"
  and .services.agent.environment.DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_MODEL_MODE == "ai_sdk"
  and .services.agent.environment.DIPOLE_AGENT_MODEL_PROVIDER == "openai_compatible"
  and .services.agent.environment.DIPOLE_AGENT_MODEL_PROVIDER_NAME == "openai"
  and .services.agent.environment.DIPOLE_AGENT_MODEL_ROUTES == "openai/gpt-5-mini"
  and .services.agent.environment.DIPOLE_AGENT_CONTEXT_COMPILER_VERSION == "v2"
  and .services.agent.environment.DIPOLE_AGENT_MEMORY_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_RETRIEVAL_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_ENABLED == "true"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_ADDRESS == "temporal:7233"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_NAMESPACE == "dipole"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_TASK_QUEUE == "dipole-agent-active-compose-check"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE == "read_active"
  and .services.agent.environment.DIPOLE_AGENT_CONTROL_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_MCP_SERVER_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_EXTERNAL_MCP_ENABLED == "false"
  and any(.services.agent.volumes[]; (.source | endswith("/tmp/dipole-agent-release-manifest-check.json"))
    and .target == "/run/dipole/release/manifest.json" and .read_only == true)
' <<<"${active_agent_config}" >/dev/null

interactive_shadow_config="$(
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
  DIPOLE_AGENT_MODEL_PROVIDER_NAME=openai \
  DIPOLE_AGENT_MODEL_BASE_URL=https://models.example.test/v1 \
  DIPOLE_AGENT_MODEL_API_KEY=compose-check-model-key \
  DIPOLE_AGENT_MODEL_ROUTES=openai/gpt-5-mini \
  DIPOLE_AGENT_CONTEXT_COMPILER_VERSION=v2 \
  DIPOLE_AGENT_MODEL_CONTEXT_PROFILES='[{"route":"openai/gpt-5-mini","contextWindowTokens":32768,"utf8BytesPerToken":3,"safetyMarginBps":1500}]' \
  DIPOLE_AGENT_MODEL_MAX_CALLS=2 \
  DIPOLE_AGENT_MODEL_TOTAL_TIMEOUT_MS=15000 \
  DIPOLE_AGENT_MODEL_MAX_OUTPUT_TOKENS=512 \
  DIPOLE_GATEWAY_AGENT_CONTROL_SECRET=compose-check-control-secret \
    docker compose \
      -f deploy/compose/docker-compose.microservices.yml \
      -f deploy/microservices/agent-ai-sdk-shadow.yml \
      -f deploy/microservices/agent-temporal-read-shadow.yml \
      -f deploy/microservices/agent-interactive-shadow.yml \
      -f deploy/microservices/agent-deepseek-v4-flash-shadow.yml config --format json
)"
jq -e '
  .services.agent.environment.DIPOLE_AGENT_RUNTIME_MODE == "shadow"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_ENABLED == "true"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE == "read_shadow"
  and .services.agent.environment.DIPOLE_AGENT_CONTROL_ENABLED == "true"
  and .services.agent.environment.DIPOLE_AGENT_MCP_SERVER_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_EXTERNAL_MCP_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_MEMORY_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_RETRIEVAL_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_MODEL_STRUCTURED_OUTPUTS == "false"
  and .services.agent.environment.DIPOLE_AGENT_MODEL_OUTPUT_MODE == "json_text"
  and .services.agent.environment.DIPOLE_AGENT_MODEL_THINKING_MODE == "disabled"
  and .services.gateway.environment.DIPOLE_GATEWAY_AGENT_CONTROL_ENABLED == "true"
  and .services.gateway.environment.DIPOLE_GATEWAY_AGENT_CONTROL_SECRET == "compose-check-control-secret"
  and .services.gateway.environment.DIPOLE_GATEWAY_AGENT_DEFINITION_ENABLED == "true"
  and .services.gateway.environment.DIPOLE_GATEWAY_AGENT_SUBSCRIPTION_ENABLED == "false"
  and .services.gateway.environment.DIPOLE_GATEWAY_AGENT_MCP_ENABLED == "false"
' <<<"${interactive_shadow_config}" >/dev/null

subscription_shadow_config="$(
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
    docker compose \
      -f deploy/compose/docker-compose.microservices.yml \
      -f deploy/microservices/agent-subscription-shadow.yml config --format json
)"
jq -e '
  .services.agent.environment.DIPOLE_AGENT_RUNTIME_MODE == "shadow"
  and .services.agent.environment.DIPOLE_AGENT_TRIGGER_MODE == "direct_target"
  and .services.agent.environment.DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED == "true"
  and .services.agent.environment.DIPOLE_AGENT_MEMORY_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_RETRIEVAL_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_CONTROL_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_MCP_SERVER_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_EXTERNAL_MCP_ENABLED == "false"
  and .services.gateway.environment.DIPOLE_GATEWAY_AGENT_DEFINITION_ENABLED == "true"
  and .services.gateway.environment.DIPOLE_GATEWAY_AGENT_SUBSCRIPTION_ENABLED == "true"
  and .services.gateway.environment.DIPOLE_GATEWAY_AGENT_CONTROL_ENABLED == "false"
  and .services.gateway.environment.DIPOLE_GATEWAY_AGENT_MCP_ENABLED == "false"
' <<<"${subscription_shadow_config}" >/dev/null

interactive_active_config="$({
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
  DIPOLE_AGENT_RELEASE_MANIFEST_FILE=/tmp/dipole-agent-release-manifest-check.json \
  DIPOLE_AGENT_CANDIDATE_VERSION=agent-runtime@compose-check \
  DIPOLE_AGENT_ACTIVE_KAFKA_GROUP_ID=dipole-agent-active-compose-check \
  DIPOLE_AGENT_MODEL_PROVIDER_NAME=openai \
  DIPOLE_AGENT_MODEL_BASE_URL=https://models.example.test/v1 \
  DIPOLE_AGENT_MODEL_API_KEY=compose-check-model-key \
  DIPOLE_AGENT_MODEL_ROUTES=openai/gpt-5-mini \
  DIPOLE_AGENT_MODEL_CONTEXT_PROFILES='[{"route":"openai/gpt-5-mini","contextWindowTokens":32768,"utf8BytesPerToken":3,"safetyMarginBps":1500}]' \
  DIPOLE_AGENT_TEMPORAL_ADDRESS=temporal:7233 \
  DIPOLE_AGENT_TEMPORAL_NAMESPACE=dipole \
  DIPOLE_AGENT_TEMPORAL_TASK_QUEUE=dipole-agent-active-compose-check \
  DIPOLE_AGENT_INTERACTIVE_TASK_QUEUE=dipole-agent-interactive-compose-check \
  DIPOLE_AGENT_CONTROL_SECRET=compose-check-control-secret \
    docker compose -f deploy/compose/docker-compose.microservices.yml \
      -f deploy/microservices/agent-active.yml \
      -f deploy/microservices/agent-interactive-active.yml config --format json
})"
jq -e '
  .services.agent.environment.DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE == "interactive_active"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_TASK_QUEUE == "dipole-agent-interactive-compose-check"
  and .services.agent.environment.DIPOLE_AGENT_CONTROL_ENABLED == "true"
  and .services.agent.environment.DIPOLE_AGENT_INTERACTIVE_MESSAGE_WRITE_ENABLED == "true"
  and .services.gateway.environment.DIPOLE_GATEWAY_AGENT_CONTROL_ENABLED == "true"
  and .services.gateway.environment.DIPOLE_GATEWAY_AGENT_DEFINITION_ENABLED == "true"
  and .services.gateway.environment.DIPOLE_GATEWAY_AGENT_SUBSCRIPTION_ENABLED == "false"
  and .services.gateway.environment.DIPOLE_GATEWAY_AGENT_ARTIFACT_ENABLED == "false"
  and .services.gateway.environment.DIPOLE_GATEWAY_AGENT_MCP_ENABLED == "false"
' <<<"${interactive_active_config}" >/dev/null

remote_gpu_mysql_aio_config="$({
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
    docker compose \
      -f deploy/compose/docker-compose.microservices.yml \
      -f deploy/microservices/remote-gpu-mysql-aio-compat.yml config --format json
})"
jq -e '
  .services.mysql.command == [
    "--character-set-server=utf8mb4",
    "--collation-server=utf8mb4_unicode_ci",
    "--default-time-zone=+00:00",
    "--innodb-use-native-aio=0"
  ]
' <<<"${remote_gpu_mysql_aio_config}" >/dev/null

promotion_agent_config="$({
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
  DIPOLE_AGENT_RELEASE_MANIFEST_FILE=/tmp/dipole-agent-release-manifest-check.json \
  DIPOLE_AGENT_CANDIDATE_VERSION=agent-runtime@compose-check \
  DIPOLE_AGENT_ACTIVE_KAFKA_GROUP_ID=dipole-agent-active-memory-promotion-compose-check \
  DIPOLE_AGENT_MODEL_PROVIDER_NAME=openai \
  DIPOLE_AGENT_MODEL_BASE_URL=https://models.example.test/v1 \
  DIPOLE_AGENT_MODEL_API_KEY=compose-check-model-key \
  DIPOLE_AGENT_MODEL_ROUTES=openai/gpt-5-mini \
  DIPOLE_AGENT_MODEL_CONTEXT_PROFILES='[{"route":"openai/gpt-5-mini","contextWindowTokens":32768,"utf8BytesPerToken":3,"safetyMarginBps":1500}]' \
  DIPOLE_AGENT_TEMPORAL_ADDRESS=temporal:7233 \
  DIPOLE_AGENT_TEMPORAL_NAMESPACE=dipole \
  DIPOLE_AGENT_TEMPORAL_TASK_QUEUE=dipole-agent-memory-promotion-compose-check \
  DIPOLE_AGENT_MEMORY_PROMOTION_AUTHORITY=operator_approved \
    docker compose -f deploy/compose/docker-compose.microservices.yml \
      -f deploy/microservices/agent-active.yml \
      -f deploy/microservices/agent-memory-promotion.yml config --format json
})"
jq -e '
  .services.core.environment.DIPOLE_INTERNAL_RPC_AGENT_MEMORY_PROMOTION_RECEIPT_COMMIT_ENABLED == "true"
  and .services.agent.environment.DIPOLE_AGENT_RUNTIME_MODE == "remote"
  and .services.agent.environment.DIPOLE_AGENT_KAFKA_GROUP_ID == "dipole-agent-active-memory-promotion-compose-check"
  and (.services.agent.environment.DIPOLE_AGENT_KAFKA_GROUP_ID | startswith("dipole-agent-active-"))
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_ENABLED == "true"
  and (.services.agent.environment.DIPOLE_AGENT_TEMPORAL_TASK_QUEUE | startswith("dipole-agent-memory-promotion-"))
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE == "promotion_active"
  and .services.agent.environment.DIPOLE_AGENT_MEMORY_PROMOTION_COMMIT_ENABLED == "true"
  and .services.agent.environment.DIPOLE_AGENT_MEMORY_PROMOTION_AUTHORITY == "operator_approved"
  and .services.agent.environment.DIPOLE_AGENT_MEMORY_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_RETRIEVAL_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_CONTROL_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_MCP_SERVER_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_EXTERNAL_MCP_ENABLED == "false"
' <<<"${promotion_agent_config}" >/dev/null

if env -u DIPOLE_AGENT_MEMORY_PROMOTION_AUTHORITY \
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
  DIPOLE_AGENT_RELEASE_MANIFEST_FILE=/tmp/dipole-agent-release-manifest-check.json \
  DIPOLE_AGENT_CANDIDATE_VERSION=agent-runtime@compose-check \
  DIPOLE_AGENT_ACTIVE_KAFKA_GROUP_ID=dipole-agent-active-memory-promotion-compose-check \
  DIPOLE_AGENT_MODEL_PROVIDER_NAME=openai \
  DIPOLE_AGENT_MODEL_BASE_URL=https://models.example.test/v1 \
  DIPOLE_AGENT_MODEL_API_KEY=compose-check-model-key \
  DIPOLE_AGENT_MODEL_ROUTES=openai/gpt-5-mini \
  DIPOLE_AGENT_MODEL_CONTEXT_PROFILES='[{"route":"openai/gpt-5-mini","contextWindowTokens":32768,"utf8BytesPerToken":3,"safetyMarginBps":1500}]' \
  DIPOLE_AGENT_TEMPORAL_ADDRESS=temporal:7233 \
  DIPOLE_AGENT_TEMPORAL_NAMESPACE=dipole \
  DIPOLE_AGENT_TEMPORAL_TASK_QUEUE=dipole-agent-memory-promotion-compose-check \
  docker compose -f deploy/compose/docker-compose.microservices.yml \
    -f deploy/microservices/agent-active.yml \
    -f deploy/microservices/agent-memory-promotion.yml config --quiet >/dev/null 2>&1; then
  echo "Agent Memory promotion overlay must reject missing operator authority" >&2
  exit 1
fi

if env -u DIPOLE_AGENT_RELEASE_MANIFEST_FILE -u DIPOLE_AGENT_CANDIDATE_VERSION \
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
  docker compose -f deploy/compose/docker-compose.microservices.yml \
    -f deploy/microservices/agent-active.yml config --quiet >/dev/null 2>&1; then
  echo "active Agent overlay must reject missing manifest and candidate inputs" >&2
  exit 1
fi

external_mcp_shadow_config="$({
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
  DIPOLE_AGENT_EXTERNAL_MCP_IO_MANIFEST_FILE=/tmp/dipole-agent-external-mcp-io-check.json \
  DIPOLE_AGENT_EXTERNAL_MCP_ROUTE_MANIFEST_FILE=/tmp/dipole-agent-external-mcp-routes-check.json \
  DIPOLE_AGENT_EXTERNAL_MCP_SECRET_DIR=/tmp/dipole-agent-external-mcp-secrets-check \
  DIPOLE_AGENT_EXTERNAL_MCP_KAFKA_BROKERS=kafka:9092 \
  DIPOLE_AGENT_EXTERNAL_MCP_KAFKA_GROUP_ID=dipole-agent-external-mcp-compose-check \
  DIPOLE_AGENT_EXTERNAL_MCP_PROFILES='[{"schema_version":"dipole.agent.external-mcp-profile.v1","profile_id":"repository-prod","tenant_id":"dipole","server_id":"repository.example","endpoint":"https://repository.example/mcp","credential":{"ref":"CRED-0123456789ABCDEF","version":1},"network_policy":{"allowed_hosts":["repository.example"],"allowed_ports":[443],"dns_resolution":"public_only","tls_server_name":"repository.example","ca_bundle_ref":"CA-0123456789ABCDEF"},"allowed_tools":["get_issue"]}]' \
  DIPOLE_AGENT_TEMPORAL_ADDRESS=temporal:7233 \
  DIPOLE_AGENT_TEMPORAL_NAMESPACE=dipole \
  DIPOLE_AGENT_TEMPORAL_TASK_QUEUE=dipole-agent-external-mcp-compose-check \
    docker compose -f deploy/compose/docker-compose.microservices.yml \
      -f deploy/microservices/agent-external-mcp-shadow.yml config --format json
})"
jq -e '
  .services.agent.environment.DIPOLE_AGENT_RUNTIME_MODE == "shadow"
  and .services.agent.environment.DIPOLE_AGENT_KAFKA_ENABLED == "true"
  and .services.agent.environment.DIPOLE_AGENT_KAFKA_BROKERS == "kafka:9092"
  and .services.agent.environment.DIPOLE_AGENT_KAFKA_GROUP_ID == "dipole-agent-external-mcp-compose-check"
  and .services.agent.environment.DIPOLE_AGENT_TRIGGER_MODE == "subscription"
  and .services.agent.environment.DIPOLE_AGENT_SUBSCRIPTION_SHADOW_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_MODEL_MODE == "metadata"
  and .services.agent.environment.DIPOLE_AGENT_MEMORY_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_RETRIEVAL_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_CAPABILITY_RPC_ENABLED == "true"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_ENABLED == "true"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_ADDRESS == "temporal:7233"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_NAMESPACE == "dipole"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_TASK_QUEUE == "dipole-agent-external-mcp-compose-check"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE == "external_mcp_shadow"
  and .services.agent.environment.DIPOLE_AGENT_CONTROL_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_MCP_SERVER_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_EXTERNAL_MCP_ENABLED == "true"
  and .services.agent.environment.DIPOLE_AGENT_EXTERNAL_MCP_IO_MANIFEST == "/run/dipole/external-mcp/io-manifest.json"
  and .services.agent.environment.DIPOLE_AGENT_EXTERNAL_MCP_ROUTE_MANIFEST == "/run/dipole/external-mcp/routes.json"
  and any(.services.agent.volumes[]; (.source | endswith("/tmp/dipole-agent-external-mcp-io-check.json"))
    and .target == "/run/dipole/external-mcp/io-manifest.json" and .read_only == true)
  and any(.services.agent.volumes[]; (.source | endswith("/tmp/dipole-agent-external-mcp-routes-check.json"))
    and .target == "/run/dipole/external-mcp/routes.json" and .read_only == true)
  and any(.services.agent.volumes[]; (.source | endswith("/tmp/dipole-agent-external-mcp-secrets-check"))
    and .target == "/run/dipole/external-mcp/secrets" and .read_only == true)
' <<<"${external_mcp_shadow_config}" >/dev/null

read_shadow_config="$({
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
  DIPOLE_AGENT_MODEL_PROVIDER_NAME=openai \
  DIPOLE_AGENT_MODEL_BASE_URL=https://models.example.test/v1 \
  DIPOLE_AGENT_MODEL_API_KEY=compose-check-model-key \
  DIPOLE_AGENT_MODEL_STRUCTURED_OUTPUTS=false \
  DIPOLE_AGENT_MODEL_OUTPUT_MODE=json_text \
  DIPOLE_AGENT_MODEL_ROUTES=openai/gpt-5-mini \
  DIPOLE_AGENT_MODEL_CONTEXT_PROFILES='[{"route":"openai/gpt-5-mini","contextWindowTokens":32768,"utf8BytesPerToken":3,"safetyMarginBps":1500}]' \
  DIPOLE_AGENT_MODEL_MAX_CALLS=1 \
  DIPOLE_AGENT_MODEL_TOTAL_TIMEOUT_MS=30000 \
  DIPOLE_AGENT_MODEL_MAX_OUTPUT_TOKENS=512 \
  DIPOLE_AGENT_CONTEXT_COMPILER_VERSION=v2 \
    docker compose -f deploy/compose/docker-compose.microservices.yml \
      -f deploy/microservices/agent-ai-sdk-shadow.yml \
      -f deploy/microservices/agent-temporal-read-shadow.yml config --format json
})"
jq -e '
  .services.temporal.image == "temporalio/auto-setup:1.29.1"
  and .services.temporal.ports == null
  and .services.temporal.environment.DB == "postgres12"
  and .services.temporal.environment.BIND_ON_IP == "0.0.0.0"
  and .services.temporal.depends_on["temporal-postgresql"].condition == "service_healthy"
  and .services["temporal-postgresql"].image == "postgres:16"
  and .services.agent.depends_on.temporal.condition == "service_healthy"
  and .services.agent.environment.DIPOLE_AGENT_MODEL_MODE == "ai_sdk"
  and .services.agent.environment.DIPOLE_AGENT_CONTEXT_COMPILER_VERSION == "v2"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_ENABLED == "true"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_ADDRESS == "temporal:7233"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_NAMESPACE == "default"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_TASK_QUEUE == "dipole-agent-read-shadow-v1"
  and .services.agent.environment.DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE == "read_shadow"
  and .services.agent.environment.DIPOLE_AGENT_MEMORY_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_RETRIEVAL_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_RETRIEVAL_CONTEXT_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_CONTROL_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_MCP_SERVER_ENABLED == "false"
  and .services.agent.environment.DIPOLE_AGENT_EXTERNAL_MCP_ENABLED == "false"
' <<<"${read_shadow_config}" >/dev/null

if env -u DIPOLE_AGENT_EXTERNAL_MCP_PROFILES \
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
  DIPOLE_AGENT_EXTERNAL_MCP_IO_MANIFEST_FILE=/tmp/dipole-agent-external-mcp-io-check.json \
  DIPOLE_AGENT_EXTERNAL_MCP_ROUTE_MANIFEST_FILE=/tmp/dipole-agent-external-mcp-routes-check.json \
  DIPOLE_AGENT_EXTERNAL_MCP_SECRET_DIR=/tmp/dipole-agent-external-mcp-secrets-check \
  DIPOLE_AGENT_EXTERNAL_MCP_KAFKA_BROKERS=kafka:9092 \
  DIPOLE_AGENT_EXTERNAL_MCP_KAFKA_GROUP_ID=dipole-agent-external-mcp-compose-check \
  DIPOLE_AGENT_TEMPORAL_ADDRESS=temporal:7233 \
  DIPOLE_AGENT_TEMPORAL_NAMESPACE=dipole \
  DIPOLE_AGENT_TEMPORAL_TASK_QUEUE=dipole-agent-external-mcp-compose-check \
  docker compose -f deploy/compose/docker-compose.microservices.yml \
    -f deploy/microservices/agent-external-mcp-shadow.yml config --quiet >/dev/null 2>&1; then
  echo "external MCP Shadow overlay must reject a missing profile input" >&2
  exit 1
fi

if DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
  DIPOLE_AGENT_RELEASE_MANIFEST_FILE=/tmp/dipole-agent-release-manifest-check.json \
  DIPOLE_AGENT_CANDIDATE_VERSION=agent-runtime@compose-check \
  docker compose -f deploy/compose/docker-compose.microservices.yml \
    -f deploy/microservices/agent-active.yml config --quiet >/dev/null 2>&1; then
  echo "active Agent overlay must reject missing provider, Temporal, context, and active Kafka inputs" >&2
  exit 1
fi

if env -u DIPOLE_AGENT_CANDIDATE_VERSION \
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
  DIPOLE_AGENT_RELEASE_MANIFEST_FILE=/tmp/dipole-agent-release-manifest-check.json \
  docker compose -f deploy/compose/docker-compose.microservices.yml \
    -f deploy/microservices/agent-active.yml config --quiet >/dev/null 2>&1; then
  echo "active Agent overlay must reject a missing candidate input" >&2
  exit 1
fi

primary_hydration_config="$({
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
  DIPOLE_CASSANDRA_ENABLED=true \
  DIPOLE_CASSANDRA_HOSTS=cassandra:9042 \
  DIPOLE_SYNC_CASSANDRA_PRIMARY_HYDRATION=true \
    docker compose -f deploy/compose/docker-compose.microservices.yml config --format json
})"
jq -e '
  .services.sync.environment.DIPOLE_CASSANDRA_ENABLED == "true"
  and .services.sync.environment.DIPOLE_CASSANDRA_HOSTS == "cassandra:9042"
  and .services.sync.environment.DIPOLE_SYNC_CASSANDRA_PRIMARY_HYDRATION == "true"
' <<<"${primary_hydration_config}" >/dev/null

primary_profile_config="$({
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
    docker compose --profile cassandra-primary \
      -f deploy/compose/docker-compose.microservices.yml \
      -f deploy/microservices/cassandra-primary.yml config --format json
})"
jq -e '
  .services.cassandra.profiles == ["cassandra-primary"]
  and .services["cassandra-init"].depends_on.cassandra.condition == "service_healthy"
  and .services["cassandra-init"].command == ["cqlsh cassandra -f /schema/001_timeline.cql"]
  and .services.sync.depends_on["cassandra-init"].condition == "service_completed_successfully"
  and any(.services.sync.volumes[]; (.source | endswith("/configs/config.cassandra-primary.yaml")) and .target == "/app/configs/config.yaml")
  and .services.core.environment.DIPOLE_GATEWAY_MODE == "embedded"
  and .services.core.environment.DIPOLE_CORE_MESSAGE_TRANSPORT == "local"
  and .services.message.depends_on.core.condition == "service_healthy"
  and .services.sync.environment.DIPOLE_CASSANDRA_ENABLED == "true"
  and .services.sync.environment.DIPOLE_CASSANDRA_HOSTS == "cassandra:9042"
  and .services.sync.environment.DIPOLE_SYNC_CASSANDRA_PRIMARY_HYDRATION == "true"
' <<<"${primary_profile_config}" >/dev/null

projector_config="$({
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
    docker compose -f deploy/compose/docker-compose.microservices.yml \
      -f deploy/microservices/inbox-projector.yml config --format json
})"
jq -e '
  .services.message.environment.DIPOLE_MESSAGE_RUNTIME_MODE == "owner"
  and .services.message.environment.DIPOLE_MESSAGE_INBOX_WRITE_MODE == "projector"
  and .services.message.environment.DIPOLE_MESSAGE_MYSQL_USER == "dipole_message_projector"
  and .services.sync.environment.DIPOLE_SYNC_PROJECTOR_ENABLED == "true"
' <<<"${projector_config}" >/dev/null

isolated_microservices_config="$({
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
  docker compose --profile search -f deploy/compose/docker-compose.microservices.yml -f deploy/microservices/isolated-images.yml config --format json
})"
jq -e '
  .services.migrate.image == "dipole-migrate:latest"
  and .services.migrate.entrypoint == ["/app/service"]
  and .services.core.image == "dipole-core:latest"
  and .services.core.entrypoint == ["/app/service"]
  and .services.gateway.image == "dipole-gateway:latest"
  and .services.gateway.entrypoint == ["/app/service"]
  and .services.message.image == "dipole-message:latest"
  and .services.message.entrypoint == ["/app/service"]
  and .services.sync.image == "dipole-sync:latest"
  and .services.sync.entrypoint == ["/app/service"]
  and .services.search.image == "dipole-search:latest"
  and .services.search.entrypoint == ["/app/service"]
  and .services["search-indexer"].image == "dipole-search-indexer:latest"
  and .services["search-indexer"].entrypoint == ["/app/service"]
' <<<"${isolated_microservices_config}" >/dev/null

cpp_microservices_config="$(
  DIPOLE_REALTIME_DELIVERY=cpp \
  DIPOLE_DELIVERY_PRIMARY_ENABLED=true \
  DIPOLE_REALTIME_FENCING_ENABLED=true \
  DIPOLE_REALTIME_FENCING_EPOCH=7 \
    docker compose --profile realtime-cpp -f deploy/compose/docker-compose.microservices.yml config --format json
)"
jq -e '
  .services["realtime-cpp"].profiles == ["realtime-cpp"]
  and .services["realtime-cpp"].environment.DIPOLE_REALTIME_DELIVERY == "cpp"
  and .services["realtime-cpp"].environment.DIPOLE_REALTIME_PRIMARY_ENABLED == "true"
  and .services["realtime-cpp"].environment.DIPOLE_REALTIME_FENCING_EPOCH == "7"
  and .services["realtime-cpp"].environment.DIPOLE_REALTIME_NODE_TRANSPORT_MODE == "primary"
  and .services.gateway.environment.DIPOLE_REALTIME_DELIVERY == "cpp"
  and .services.gateway.environment.DIPOLE_INTERNAL_RPC_DELIVERY_PRIMARY_ENABLED == "true"
' <<<"${cpp_microservices_config}" >/dev/null

agent_mcp_drill_config="$(
  DIPOLE_AGENT_DRILL_MYSQL_PORT=23306 \
  DIPOLE_AGENT_DRILL_KAFKA_PORT=29092 \
    docker compose -f deploy/agent/external-mcp-shadow-drill.compose.yml config --format json
)"
jq -e '
  .services.mysql.command == [
    "--character-set-server=utf8mb4",
    "--collation-server=utf8mb4_unicode_ci",
    "--innodb-use-native-aio=0"
  ]
' <<<"${agent_mcp_drill_config}" >/dev/null

candidate_config="$({
  DIPOLE_CONTAINER_PREFIX=candidate-compose-validation-only \
  DIPOLE_IMAGE=sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc \
  DIPOLE_MYSQL_PORT=13306 \
  DIPOLE_REDIS_PORT=16379 \
  DIPOLE_KAFKA_EXTERNAL_PORT=19094 \
  DIPOLE_KAFDROP_PORT=19099 \
  DIPOLE_MINIO_PORT=19000 \
  DIPOLE_MINIO_CONSOLE_PORT=19001 \
  DIPOLE_NODE1_PORT=18081 \
  DIPOLE_NODE2_PORT=18082 \
  DIPOLE_NODE3_PORT=18083 \
  DIPOLE_HTTP_PORT=18080 \
  DIPOLE_HTTPS_PORT=18443 \
  DIPOLE_NETWORK_SUBNET=10.201.0.0/24 \
  DIPOLE_AI_RUNTIME_MODE=off \
    docker compose -f deploy/compose/docker-compose.dist.yml config --format json
})"

jq -e '
  .services.mysql.container_name == "candidate-compose-validation-only-mysql"
  and .services.redis.ports[0].published == "16379"
  and .services["dipole-node1"].image == "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
  and .services["dipole-node1"].ports[0].published == "18081"
  and .services["dipole-node2"].ports[0].published == "18082"
  and .services["dipole-node3"].ports[0].published == "18083"
  and .services["dipole-node1"].environment.DIPOLE_AI_RUNTIME_MODE == "off"
  and .services.nginx.ports[0].published == "18080"
  and .networks.default.ipam.config[0].subnet == "10.201.0.0/24"
' <<<"${candidate_config}" >/dev/null
