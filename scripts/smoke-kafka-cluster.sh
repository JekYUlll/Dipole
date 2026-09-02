#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/deploy/compose/docker-compose.cluster.yml"
PROJECT_NAME="${COMPOSE_PROJECT_NAME:-dipole-kafka-cluster-smoke}"
TOPIC="${KAFKA_SMOKE_TOPIC:-dipole.cluster.smoke}"
BROKERS="kafka-1:9092,kafka-2:9092,kafka-3:9092"
NO_QUORUM_LOG="$(mktemp)"

compose() {
  docker compose -p "${PROJECT_NAME}" -f "${COMPOSE_FILE}" "$@"
}

cleanup() {
	rm -f "${NO_QUORUM_LOG}"
	if [[ "${KEEP_STACK:-0}" != "1" ]]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

kafka_exec() {
  compose exec -T kafka-1 "/opt/kafka/bin/$1" "${@:2}"
}

wait_for_min_isr() {
  local minimum="$1"
  local deadline=$((SECONDS + 90))
  while (( SECONDS < deadline )); do
    local description
    description="$(kafka_exec kafka-topics.sh --bootstrap-server "${BROKERS}" --describe --topic "${TOPIC}" 2>/dev/null || true)"
    local ready=1
    local partitions=0
    while IFS= read -r line; do
      [[ "${line}" == *"Partition:"* ]] || continue
      partitions=$((partitions + 1))
      local isr
      isr="$(sed -n 's/.*Isr: \([^[:space:]]*\).*/\1/p' <<<"${line}")"
      local count=0
      [[ -z "${isr}" ]] || count=$(( $(tr -cd ',' <<<"${isr}" | wc -c) + 1 ))
      if (( count < minimum )); then
        ready=0
      fi
    done <<<"${description}"
    if (( partitions == 3 && ready == 1 )); then
      return 0
    fi
    sleep 2
  done
  echo "Kafka topic ${TOPIC} did not reach min ISR ${minimum}" >&2
  return 1
}

produce() {
  local value="$1"
  printf '%s\n' "${value}" | kafka_exec kafka-console-producer.sh \
    --bootstrap-server "${BROKERS}" \
    --topic "${TOPIC}" \
    --producer-property acks=all \
    --producer-property delivery.timeout.ms=5000 \
    --producer-property request.timeout.ms=3000 \
    --producer-property max.block.ms=5000
}

compose config --quiet
compose up -d --wait

kafka_exec kafka-topics.sh --bootstrap-server "${BROKERS}" --create \
  --topic "${TOPIC}" --partitions 3 --replication-factor 3 \
  --config min.insync.replicas=2 --config retention.ms=3600000
wait_for_min_isr 3

produce "before-failure"
compose stop kafka-3
wait_for_min_isr 2
produce "single-broker-failure"

compose stop kafka-2
sleep 8
set +e
produce "must-not-ack-without-quorum" >"${NO_QUORUM_LOG}" 2>&1
no_quorum_status=$?
set -e
if ! grep -Eq "NOT_ENOUGH_REPLICAS|NotEnoughReplicas" "${NO_QUORUM_LOG}"; then
	echo "Kafka did not report the expected below-min-ISR rejection (exit ${no_quorum_status})" >&2
	cat "${NO_QUORUM_LOG}" >&2
	exit 1
fi

compose start kafka-2
wait_for_min_isr 2
produce "after-recovery"

messages="$(kafka_exec kafka-console-consumer.sh --bootstrap-server "${BROKERS}" \
  --topic "${TOPIC}" --from-beginning --max-messages 3 --timeout-ms 30000)"
for expected in before-failure single-broker-failure after-recovery; do
  grep -qx "${expected}" <<<"${messages}"
done
if grep -q "must-not-ack-without-quorum" <<<"${messages}"; then
  echo "Rejected no-quorum write appeared in the log" >&2
  exit 1
fi

echo "Kafka cluster smoke passed: RF=3, min ISR=2, one-broker survival, quorum rejection, and recovery"
