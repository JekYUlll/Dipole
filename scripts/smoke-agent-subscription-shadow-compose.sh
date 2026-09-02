#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
project_name="${COMPOSE_PROJECT_NAME:-dipole-agent-subscription-shadow-${RANDOM}-$$}"
scratch_dir=$(mktemp -d "${TMPDIR:-/tmp}/dipole-agent-subscription-shadow.XXXXXX")
owner_telephone="13900000003"
agent_uuid="UAI000000000000000001"
event_id="SUBSCRIPTION-SHADOW-SMOKE-${RANDOM}-$$"

compose_files=(
  -f "${root_dir}/deploy/compose/docker-compose.microservices.yml"
  -f "${root_dir}/deploy/microservices/agent-subscription-shadow.yml"
)
[[ "${DIPOLE_MYSQL_AIO_COMPAT:-0}" == "0" ]] || compose_files+=(-f "${root_dir}/deploy/microservices/remote-gpu-mysql-aio-compat.yml")
compose() { docker compose -p "${project_name}" "${compose_files[@]}" "$@"; }

cleanup() {
  local status=$?
  compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -rf "${scratch_dir}"
  exit "${status}"
}
trap cleanup EXIT INT TERM

export DIPOLE_INTERNAL_RPC_SHARED_SECRET="${DIPOLE_INTERNAL_RPC_SHARED_SECRET:-$(openssl rand -hex 32)}"
export DIPOLE_INTERNAL_CERT_DIR="${scratch_dir}/certs"
export INTERNAL_CERT_DIR="${DIPOLE_INTERNAL_CERT_DIR}"
export DIPOLE_GATEWAY_BIND_ADDRESS="${DIPOLE_GATEWAY_BIND_ADDRESS:-127.0.0.1}"
export DIPOLE_GATEWAY_PORT="${DIPOLE_GATEWAY_PORT:-$((18000 + RANDOM % 2000))}"
"${root_dir}/scripts/generate-internal-certs.sh"
compose config --quiet
compose up -d --wait

owner_uuid=$(compose exec -T agent node --input-type=module - "${owner_telephone}" "${agent_uuid}" <<'NODE'
import { Kafka } from "kafkajs";
const [telephone, agentUuid] = process.argv.slice(2);
const register = await fetch("http://core:8081/api/v1/auth/register", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ nickname: "Shadow", telephone, password: "smoke-pass-123" }) });
const registered = await register.json();
if (register.status !== 200 || typeof registered?.data?.user?.uuid !== "string") throw new Error(`register failed: ${register.status} ${JSON.stringify(registered)}`);
const ownerUuid = registered.data.user.uuid;
const login = await fetch("http://core:8081/api/v1/auth/login", { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify({ telephone, password: "smoke-pass-123" }) });
const token = (await login.json())?.data?.token;
if (login.status !== 200 || typeof token !== "string") throw new Error(`login failed: ${login.status}`);
const headers = { authorization: `Bearer ${token}`, "content-type": "application/json" };
const definitionResponse = await fetch("http://gateway:8080/api/v1/agent/definitions", { method: "POST", headers });
const definition = await definitionResponse.json();
if (definitionResponse.status !== 201 || typeof definition?.definitionId !== "string" || definition.version !== 1) throw new Error(`definition failed: ${definitionResponse.status}`);
const conversationKey = [ownerUuid, agentUuid].sort().join(":");
const subscriptionResponse = await fetch("http://gateway:8080/api/v1/agent/subscriptions", { method: "POST", headers, body: JSON.stringify({ definitionId: definition.definitionId, definitionVersion: definition.version, conversationKey: `direct:${conversationKey}`, filterKind: "all", filter: {} }) });
const subscription = await subscriptionResponse.json();
if (subscriptionResponse.status !== 200 || typeof subscription?.subscriptionId !== "string" || subscription.status !== "active") throw new Error(`subscription failed: ${subscriptionResponse.status} ${JSON.stringify(subscription)}`);
process.stdout.write(`${ownerUuid}\t${subscription.subscriptionId}`);
NODE
)
IFS=$'\t' read -r owner_uuid subscription_uuid <<<"${owner_uuid}"

compose exec -T agent node --input-type=module - "${event_id}" "${owner_uuid}" "${agent_uuid}" <<'NODE'
import { Kafka } from "kafkajs";
const [eventId, ownerUuid, agentUuid] = process.argv.slice(2);
const conversationKey = `direct:${[ownerUuid, agentUuid].sort().join(":")}`;
const event = { event_id: eventId, event_type: "message.direct.created", version: "v1", source: "dipole", occurred_at: new Date().toISOString(), payload: { mutation_type: "created", revision: 1, actor_uuid: ownerUuid, message_id: `M-${eventId}`, conversation_key: conversationKey, message_seq: 1, sender_uuid: ownerUuid, target_uuid: agentUuid, target_type: 0, message_type: 0, content: "subscription shadow smoke", sent_at: new Date().toISOString() } };
const producer = new Kafka({ clientId: "subscription-shadow-smoke", brokers: ["kafka:9092"] }).producer();
await producer.connect();
await producer.send({ topic: "dipole.message.direct.created", messages: [{ key: eventId, value: JSON.stringify(event) }] });
await producer.disconnect();
NODE

for _ in $(seq 1 60); do
  metrics=$(compose exec -T agent node --input-type=module - <<'NODE'
const text = await (await fetch("http://127.0.0.1:8091/metrics")).text();
process.stdout.write(text);
NODE
)
  if [[ "${metrics}" == *'direct_target="accepted",subscription="match"'* || "${metrics}" == *'subscription="match",direct_target="accepted"'* ]]; then break; fi
  sleep 1
done
[[ "${metrics}" == *'direct_target="accepted",subscription="match"'* || "${metrics}" == *'subscription="match",direct_target="accepted"'* ]] || { printf 'Subscription Shadow metric did not match\n' >&2; exit 1; }
task_count=$(compose exec -T mysql mysql -N -B -uroot -proot123 dipole -e "SELECT COUNT(*) FROM agent_tasks WHERE trigger_ref = 'M-${event_id}'")
[[ "${task_count}" == "1" ]] || { printf 'expected one direct-target task, got %s\n' "${task_count}" >&2; exit 1; }
printf 'Agent Subscription Shadow Compose smoke passed: project=%s subscription=%s\n' "${project_name}" "${subscription_uuid}"
