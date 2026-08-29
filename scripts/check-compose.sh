#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=static-compose-validation-only}"
export DIPOLE_INTERNAL_RPC_SHARED_SECRET

for file in docker-compose.yml deploy/compose/docker-compose*.yml; do
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
  check_bind_sources "$file"
done

default_microservices_config="$(docker compose -f deploy/compose/docker-compose.microservices.yml config --format json)"
jq -e '
  (.services["realtime-cpp"] == null)
  and .services.gateway.environment.DIPOLE_REALTIME_DELIVERY == "go"
  and .services.gateway.environment.DIPOLE_INTERNAL_RPC_DELIVERY_PRIMARY_ENABLED == "false"
  and .services.migrate.image == "dipole-migrate:latest"
  and .services.migrate.entrypoint == ["/app/service"]
  and .services.core.image == "dipole-core:latest"
  and .services.core.entrypoint == ["/app/service"]
  and .services.core.environment.DIPOLE_CORE_MESSAGE_TRANSPORT == "local"
  and .services.core.environment.DIPOLE_MESSAGE_TRANSPORT == "grpc"
  and .services.gateway.image == "dipole-gateway:latest"
  and .services.gateway.entrypoint == ["/app/service"]
  and .services.message.image == "dipole-message:latest"
  and .services.message.entrypoint == ["/app/service"]
  and .services.sync.image == "dipole-sync:latest"
  and .services.sync.entrypoint == ["/app/service"]
  and .services.gateway.depends_on.sync.condition == "service_healthy"
' <<<"${default_microservices_config}" >/dev/null

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
