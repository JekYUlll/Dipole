#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/deploy/compose/docker-compose.cluster.yml"
PROJECT_NAME="${COMPOSE_PROJECT_NAME:-dipole-kafka-rebalance-smoke}"
TOPIC="${KAFKA_REBALANCE_TOPIC:-dipole-rebalance-smoke}"
GROUP="${KAFKA_REBALANCE_GROUP:-dipole-rebalance-group}"
BROKERS="kafka-1:9092,kafka-2:9092,kafka-3:9092"

compose() {
  docker compose -p "${PROJECT_NAME}" -f "${COMPOSE_FILE}" "$@"
}

cleanup() {
  if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

kafka_exec() {
  compose exec -T kafka-1 "/opt/kafka/bin/$1" "${@:2}"
}

start_consumer() {
  local service="$1"
  local client_id="$2"
  compose exec -d "${service}" sh -c \
    "echo \$\$ >/tmp/${client_id}.pid; exec /opt/kafka/bin/kafka-console-consumer.sh \
      --bootstrap-server ${BROKERS} --topic ${TOPIC} --group ${GROUP} \
      --consumer-property client.id=${client_id} --timeout-ms 120000 >/tmp/${client_id}.log 2>&1"
}

wait_for_member_count() {
  local expected="$1"
  local deadline=$((SECONDS + 60))
  while (( SECONDS < deadline )); do
    local members
    members="$(kafka_exec kafka-consumer-groups.sh --bootstrap-server "${BROKERS}" \
      --describe --group "${GROUP}" --members --verbose 2>/dev/null || true)"
    local count
    count="$(awk -v group="${GROUP}" '$1 == group { print $2 }' <<<"${members}" | sort -u | sed '/^$/d' | wc -l)"
    if (( count == expected )); then
      return 0
    fi
    sleep 2
  done
  echo "Consumer group ${GROUP} did not reach ${expected} members" >&2
  return 1
}

wait_for_zero_lag() {
  local deadline=$((SECONDS + 60))
  while (( SECONDS < deadline )); do
    local offsets
    offsets="$(kafka_exec kafka-consumer-groups.sh --bootstrap-server "${BROKERS}" \
      --describe --group "${GROUP}" 2>/dev/null || true)"
    if awk -v group="${GROUP}" '
      $1 == group { rows++; if ($6 != 0) lagged = 1 }
      END { exit !(rows == 6 && lagged == 0) }
    ' <<<"${offsets}"; then
      return 0
    fi
    sleep 2
  done
  echo "Consumer group ${GROUP} did not drain lag" >&2
  return 1
}

produce_batch() {
  local prefix="$1"
  local count="$2"
  seq 1 "${count}" | sed "s/^/${prefix}-/" | kafka_exec kafka-console-producer.sh \
    --bootstrap-server "${BROKERS}" --topic "${TOPIC}" --producer-property acks=all
}

compose config --quiet
compose up -d --wait
kafka_exec kafka-topics.sh --bootstrap-server "${BROKERS}" --create \
  --topic "${TOPIC}" --partitions 6 --replication-factor 3 \
  --config min.insync.replicas=2 --config retention.ms=3600000

start_consumer kafka-1 smoke-c1
start_consumer kafka-2 smoke-c2
wait_for_member_count 2
produce_batch before 60
wait_for_zero_lag

compose exec -T kafka-1 sh -c 'kill "$(cat /tmp/smoke-c1.pid)"'
wait_for_member_count 1
produce_batch after 20
wait_for_zero_lag

members="$(kafka_exec kafka-consumer-groups.sh --bootstrap-server "${BROKERS}" \
  --describe --group "${GROUP}" --members --verbose)"
grep -q "smoke-c2" <<<"${members}"
grep -Eq '[[:space:]]6[[:space:]]+\(0,1,2,3,4,5\)' <<<"${members}"

echo "Kafka rebalance smoke passed: 2 members, member loss, 6-partition takeover, and zero lag"
