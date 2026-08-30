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
grep -F "当前仓库已具备可渲染的 Kafka 三节点、Redis Sentinel 业务组合拓扑，微服务默认路径仍是单节点。" \
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
  and .services.gateway.image == "dipole-gateway:latest"
  and .services.gateway.entrypoint == ["/app/service"]
  and .services.message.image == "dipole-message:latest"
  and .services.message.entrypoint == ["/app/service"]
  and .services.sync.image == "dipole-sync:latest"
  and .services.sync.entrypoint == ["/app/service"]
  and .services.agent.image == "dipole-agent:latest"
  and (.services.agent.build.context | endswith("/services/agent-runtime"))
  and .services.agent.environment.DIPOLE_AGENT_KAFKA_ENABLED == "true"
  and .services.agent.environment.DIPOLE_AGENT_RUNTIME_MODE == "shadow"
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
    docker compose -f deploy/compose/docker-compose.microservices.yml \
      -f deploy/microservices/agent-active.yml config --format json
})"
jq -e '
  .services.agent.environment.DIPOLE_AGENT_RUNTIME_MODE == "remote"
  and .services.agent.environment.DIPOLE_AGENT_CANDIDATE_VERSION == "agent-runtime@compose-check"
  and .services.agent.environment.DIPOLE_AGENT_RELEASE_MANIFEST == "/run/dipole/release/manifest.json"
  and any(.services.agent.volumes[]; (.source | endswith("/tmp/dipole-agent-release-manifest-check.json"))
    and .target == "/run/dipole/release/manifest.json" and .read_only == true)
' <<<"${active_agent_config}" >/dev/null

if env -u DIPOLE_AGENT_RELEASE_MANIFEST_FILE -u DIPOLE_AGENT_CANDIDATE_VERSION \
  DIPOLE_INTERNAL_RPC_SHARED_SECRET=static-compose-validation-only \
  docker compose -f deploy/compose/docker-compose.microservices.yml \
    -f deploy/microservices/agent-active.yml config --quiet >/dev/null 2>&1; then
  echo "active Agent overlay must reject missing manifest and candidate inputs" >&2
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

DIPOLE_AGENT_DRILL_MYSQL_PORT=23306 \
DIPOLE_AGENT_DRILL_KAFKA_PORT=29092 \
  docker compose -f deploy/agent/external-mcp-shadow-drill.compose.yml config --quiet

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
