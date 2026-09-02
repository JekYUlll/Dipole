#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
COMPOSE_FILE="$ROOT_DIR/deploy/compose/docker-compose.cluster.yml"
PROJECT_NAME="dipole-kafka-observability-${RANDOM}"
TOPIC="dipole.observability.${RANDOM}"
GROUP="dipole-observability-${RANDOM}"

compose() {
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

cleanup() {
  compose down --volumes --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT

wait_for_query() {
  local expression=$1
  local predicate=$2
  local attempts=${3:-30}
  local result
  local value
  for ((i = 1; i <= attempts; i++)); do
    result=$(compose exec -T prometheus promtool query instant http://localhost:9090 "$expression" 2>/dev/null || true)
    value=$(awk -F'=> ' '/=>/{split($2, fields, " "); print fields[1]; exit}' <<<"$result")
    if [[ "$predicate" == "positive" && -n "$value" ]] && awk "BEGIN {exit !($value > 0)}"; then
      return 0
    fi
    if [[ "$predicate" == "zero" && -n "$value" ]] && awk "BEGIN {exit !($value == 0)}"; then
      return 0
    fi
    sleep 2
  done
  echo "query did not become $predicate: $expression" >&2
  echo "$result" >&2
  return 1
}

compose up -d kafka-1 kafka-2 kafka-3 kafka-exporter prometheus
compose exec -T prometheus promtool check config /etc/prometheus/prometheus.yml

compose exec -T kafka-1 /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server kafka-1:9092 \
  --create --if-not-exists \
  --topic "$TOPIC" --partitions 1 --replication-factor 3 \
  --config min.insync.replicas=2
compose exec -T kafka-1 /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server kafka-1:9092 \
  --create --if-not-exists \
  --topic "$TOPIC.retry" --partitions 1 --replication-factor 3 \
  --config min.insync.replicas=2
compose exec -T kafka-1 /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server kafka-1:9092 \
  --create --if-not-exists \
  --topic "$TOPIC.dead" --partitions 1 --replication-factor 3 \
  --config min.insync.replicas=2

printf 'seed\n' | compose exec -T kafka-1 /opt/kafka/bin/kafka-console-producer.sh \
  --bootstrap-server kafka-1:9092 --topic "$TOPIC"
compose exec -T kafka-1 /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server kafka-1:9092 --topic "$TOPIC" --group "$GROUP" \
  --from-beginning --max-messages 1 --timeout-ms 15000 >/dev/null
printf 'lag-1\nlag-2\n' | compose exec -T kafka-1 /opt/kafka/bin/kafka-console-producer.sh \
  --bootstrap-server kafka-1:9092 --topic "$TOPIC"
wait_for_query "sum(kafka_consumergroup_lag{consumergroup=\"$GROUP\"})" positive

printf 'retry\n' | compose exec -T kafka-1 /opt/kafka/bin/kafka-console-producer.sh \
  --bootstrap-server kafka-1:9092 --topic "$TOPIC.retry"
printf 'dead\n' | compose exec -T kafka-1 /opt/kafka/bin/kafka-console-producer.sh \
  --bootstrap-server kafka-1:9092 --topic "$TOPIC.dead"
wait_for_query "sum(kafka_topic_partition_current_offset{topic=\"$TOPIC.retry\"})" positive
wait_for_query "sum(kafka_topic_partition_current_offset{topic=\"$TOPIC.dead\"})" positive

wait_for_query "sum(kafka_topic_partition_replicas - kafka_topic_partition_in_sync_replica)" zero
compose stop kafka-3 >/dev/null
wait_for_query "sum(kafka_topic_partition_replicas - kafka_topic_partition_in_sync_replica)" positive
compose start kafka-3 >/dev/null
wait_for_query "sum(kafka_topic_partition_replicas - kafka_topic_partition_in_sync_replica)" zero 60

echo "Kafka observability smoke passed: lag, retry, DLQ, and ISR metrics are queryable."
