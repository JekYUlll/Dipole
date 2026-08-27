#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=static-compose-validation-only}"
export DIPOLE_INTERNAL_RPC_SHARED_SECRET

for file in docker-compose*.yml; do
  docker compose -f "$file" config --quiet
done

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
    docker compose -f docker-compose.dist.yml config --format json
})"

jq -e '
  .services.mysql.container_name == "candidate-compose-validation-only-mysql"
  and .services.redis.ports[0].published == "16379"
  and .services["dipole-node1"].image == "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
  and .services["dipole-node1"].ports[0].published == "18081"
  and .services["dipole-node2"].ports[0].published == "18082"
  and .services["dipole-node3"].ports[0].published == "18083"
  and .services.migrate.entrypoint == ["/app/dipole-migrate"]
  and .services["dipole-node1"].depends_on.migrate.condition == "service_completed_successfully"
  and .services["dipole-node1"].environment.DIPOLE_AI_RUNTIME_MODE == "off"
  and .services.nginx.ports[0].published == "18080"
  and .networks.default.ipam.config[0].subnet == "10.201.0.0/24"
' <<<"${candidate_config}" >/dev/null
