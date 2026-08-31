#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root_dir=$(cd "${script_dir}/.." && pwd)
project=${COMPOSE_PROJECT_NAME:-dipole-isolated-image-smoke}
gateway_port=${GATEWAY_PORT:-18080}
report_file=${SMOKE_REPORT_FILE:-/tmp/${project}-receipt.json}
message_restart_service=${SMOKE_MESSAGE_RESTART_SERVICE:-}
exec_timeout_seconds=${SMOKE_EXEC_TIMEOUT_SECONDS:-20}
wscli_log=$(mktemp -t dipole-isolated-wscli.XXXXXX.log)
wscli_binary=$(mktemp -t dipole-isolated-wscli.XXXXXX)
cert_dir=${DIPOLE_INTERNAL_CERT_DIR:-$(mktemp -d -t dipole-isolated-certs.XXXXXX)}
ports_file=$(mktemp -t dipole-isolated-ports.XXXXXX.yml)
remove_cert_dir=0
if [[ -z "${DIPOLE_INTERNAL_CERT_DIR:-}" ]]; then
  remove_cert_dir=1
fi

cat >"${ports_file}" <<EOF
services:
  gateway:
    ports: !override
      - "${gateway_port}:8080"
EOF

export DIPOLE_INTERNAL_CERT_DIR="${cert_dir}"
: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=$(openssl rand -hex 32)}"
export DIPOLE_INTERNAL_RPC_SHARED_SECRET

compose_args=(-p "${project}")
if [[ "${SMOKE_SEARCH_PROFILE:-0}" == "1" ]]; then
  compose_args+=(--profile search)
fi
compose_args+=(
  -f "${root_dir}/deploy/compose/docker-compose.microservices.yml"
  -f "${root_dir}/deploy/microservices/isolated-images.yml"
)
if [[ "${SMOKE_INBOX_PROJECTOR:-0}" == "1" ]]; then
  compose_args+=( -f "${root_dir}/deploy/microservices/inbox-projector.yml" )
fi
compose_args+=( -f "${ports_file}" )

compose() {
  docker compose "${compose_args[@]}" "$@"
}

compose_exec() {
  local service=$1
  shift
  local container_id
  container_id="$(timeout --foreground -k 5s "${exec_timeout_seconds}" docker compose "${compose_args[@]}" ps -q "${service}")"
  [[ -n "${container_id}" ]] || {
    printf 'service container is unavailable: %s\n' "${service}" >&2
    return 1
  }
  timeout --foreground -k 5s "${exec_timeout_seconds}" docker exec "${container_id}" "$@"
}

cleanup() {
  local exit_code=$?
  local retain_failure=0
  if [[ "${KEEP_ON_FAILURE:-0}" == "1" && "${exit_code}" != "0" ]]; then
    retain_failure=1
  fi
  if [[ "${retain_failure}" == "0" ]]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  else
    printf 'isolated microservices smoke retained failed project: %s\n' "${project}" >&2
  fi
  rm -f "${ports_file}"
  rm -f "${wscli_log}"
  rm -f "${wscli_binary}"
  if [[ "${remove_cert_dir}" == "1" && "${retain_failure}" == "0" ]]; then
    rm -rf "${cert_dir}"
  elif [[ "${retain_failure}" == "1" ]]; then
    printf 'isolated microservices smoke retained certificate directory: %s\n' "${cert_dir}" >&2
  fi
  exit "${exit_code}"
}
trap cleanup EXIT INT TERM

if [[ "${BUILD_IMAGE:-0}" == "1" ]]; then
  "${script_dir}/docker-build.sh" backend
  "${script_dir}/docker-build-microservice-images.sh"
fi

: "${DIPOLE_IMAGE:=dipole-server:latest}"
: "${DIPOLE_MIGRATE_IMAGE:=dipole-migrate:latest}"
: "${DIPOLE_CORE_IMAGE:=dipole-core:latest}"
: "${DIPOLE_GATEWAY_IMAGE:=dipole-gateway:latest}"
: "${DIPOLE_MESSAGE_IMAGE:=dipole-message:latest}"
: "${DIPOLE_SYNC_IMAGE:=dipole-sync:latest}"
: "${DIPOLE_SEARCH_IMAGE:=dipole-search:latest}"
: "${DIPOLE_SEARCH_INDEXER_IMAGE:=dipole-search-indexer:latest}"
: "${DIPOLE_AGENT_TIMELINE_REPAIR_IMAGE:=dipole-agent-timeline-repair:latest}"
export DIPOLE_IMAGE DIPOLE_MIGRATE_IMAGE DIPOLE_CORE_IMAGE DIPOLE_GATEWAY_IMAGE
export DIPOLE_MESSAGE_IMAGE DIPOLE_SYNC_IMAGE DIPOLE_SEARCH_IMAGE DIPOLE_SEARCH_INDEXER_IMAGE
export DIPOLE_AGENT_TIMELINE_REPAIR_IMAGE

if [[ -n "${message_restart_service}" ]]; then
  case "${message_restart_service}" in
    core|gateway|message|sync) ;;
    *)
      printf 'SMOKE_MESSAGE_RESTART_SERVICE must be one of core, gateway, message, sync\n' >&2
      exit 2
      ;;
  esac
  if [[ "${SMOKE_MESSAGE_FLOW:-0}" != "1" ]]; then
    printf 'SMOKE_MESSAGE_RESTART_SERVICE requires SMOKE_MESSAGE_FLOW=1\n' >&2
    exit 2
  fi
fi

[[ "${exec_timeout_seconds}" =~ ^[1-9][0-9]*$ ]] || {
  printf 'SMOKE_EXEC_TIMEOUT_SECONDS must be a positive integer\n' >&2
  exit 2
}

INTERNAL_CERT_DIR="${cert_dir}" "${script_dir}/generate-internal-certs.sh" >/dev/null
compose config --quiet
compose up -d

for _ in $(seq 1 120); do
  gateway_health=$(curl --fail --silent --show-error --connect-timeout 2 --max-time 5 \
    "http://127.0.0.1:${gateway_port}/health" 2>/dev/null || true)
  [[ "${gateway_health}" == *'"component":"gateway"'* ]] && break
  sleep 1
done
[[ "${gateway_health}" == *'"component":"gateway"'* ]]

for service in core message sync gateway; do
  for _ in $(seq 1 120); do
    live=$(compose_exec "${service}" wget -q -O - http://127.0.0.1:9100/livez 2>/dev/null || true)
    ready=$(compose_exec "${service}" wget -q -O - http://127.0.0.1:9100/readyz 2>/dev/null || true)
    [[ "${live}" == alive && "${ready}" == ready ]] && break
    sleep 1
  done
  test "${live}" = alive
  test "${ready}" = ready
done

if [[ "${SMOKE_SEARCH_PROFILE:-0}" == "1" ]]; then
  for service in search search-indexer; do
    compose_exec "${service}" wget -q -O - http://127.0.0.1:9100/livez | grep -qx alive
    compose_exec "${service}" wget -q -O - http://127.0.0.1:9100/readyz | grep -qx ready
  done
fi

if [[ "${SMOKE_MESSAGE_FLOW:-0}" == "1" ]]; then
  (cd "${root_dir}" && CGO_ENABLED=0 go build -o "${wscli_binary}" ./cmd/tools/wscli)
  sender_phone="138$(printf '%08d' $(((RANDOM * 32768 + RANDOM) % 100000000)))"
  target_phone="139$(printf '%08d' $(((RANDOM * 32768 + RANDOM) % 100000000)))"
  sender_password=123456
  target_password=123456
  message_content="isolated-candidate-message-${project}"
  client_message_id="isolated-candidate-client-${project}"

  register() {
    local phone=$1 nickname=$2
    curl --fail --silent --show-error --connect-timeout 2 --max-time 5 \
      -H 'Content-Type: application/json' \
      -d "{\"nickname\":\"${nickname}\",\"telephone\":\"${phone}\",\"password\":\"123456\"}" \
      "http://127.0.0.1:${gateway_port}/api/v1/auth/register"
  }

  sender_registration=$(register "${sender_phone}" SmokeSender)
  target_registration=$(register "${target_phone}" SmokeTarget)
  sender_uuid=$(jq -r '.data.user.uuid' <<<"${sender_registration}")
  target_uuid=$(jq -r '.data.user.uuid' <<<"${target_registration}")
  test -n "${sender_uuid}" -a "${sender_uuid}" != null
  test -n "${target_uuid}" -a "${target_uuid}" != null

  sender_login=$(curl --fail --silent --show-error --connect-timeout 2 --max-time 5 \
    -H 'Content-Type: application/json' \
    -d "{\"telephone\":\"${sender_phone}\",\"password\":\"${sender_password}\"}" \
    "http://127.0.0.1:${gateway_port}/api/v1/auth/login")
  sender_token=$(jq -r '.data.token' <<<"${sender_login}")
  test -n "${sender_token}" -a "${sender_token}" != null

  target_login=$(curl --fail --silent --show-error --connect-timeout 2 --max-time 5 \
    -H 'Content-Type: application/json' \
    -d "{\"telephone\":\"${target_phone}\",\"password\":\"${target_password}\"}" \
    "http://127.0.0.1:${gateway_port}/api/v1/auth/login")
  target_token=$(jq -r '.data.token' <<<"${target_login}")
  test -n "${target_token}" -a "${target_token}" != null

  application=$(curl --fail --silent --show-error --connect-timeout 2 --max-time 5 \
    -H "Authorization: Bearer ${sender_token}" -H 'Content-Type: application/json' \
    -d "{\"target_uuid\":\"${target_uuid}\",\"message\":\"isolated smoke friendship\"}" \
    "http://127.0.0.1:${gateway_port}/api/v1/contacts/applications")
  application_id=$(jq -r '.data.id' <<<"${application}")
  test -n "${application_id}" -a "${application_id}" != null
  curl --fail --silent --show-error --connect-timeout 2 --max-time 5 \
    -X PATCH -H "Authorization: Bearer ${target_token}" -H 'Content-Type: application/json' \
    -d '{"action":"accept"}' \
    "http://127.0.0.1:${gateway_port}/api/v1/contacts/applications/${application_id}" >/dev/null

  send_message() {
    local attempt=$1
    if { printf '/send %s\n' "${message_content}"; sleep 2; } |
      "${wscli_binary}" -base "http://127.0.0.1:${gateway_port}" \
        -telephone "${sender_phone}" -password "${sender_password}" -target "${target_uuid}" \
        -client-message-id "${client_message_id}" >>"${wscli_log}" 2>&1; then
      return 0
    fi
    wscli_status=$?
    if ! grep -q 'logged in as' "${wscli_log}"; then
      printf 'message attempt %s failed before login\n' "${attempt}" >&2
      cat "${wscli_log}" >&2
      return "${wscli_status}"
    fi
  }

  restart_message_service() {
    local service=$1
    compose restart "${service}"
    for _ in $(seq 1 60); do
      live=$(compose_exec "${service}" wget -q -O - http://127.0.0.1:9100/livez 2>/dev/null || true)
      ready=$(compose_exec "${service}" wget -q -O - http://127.0.0.1:9100/readyz 2>/dev/null || true)
      [[ "${live}" == alive && "${ready}" == ready ]] && break
      sleep 1
    done
    test "${live}" = alive
    test "${ready}" = ready
    if [[ "${service}" == gateway ]]; then
      for _ in $(seq 1 30); do
        gateway_health=$(curl --fail --silent --connect-timeout 2 --max-time 5 \
          "http://127.0.0.1:${gateway_port}/health" 2>/dev/null || true)
        [[ "${gateway_health}" == *'"component":"gateway"'* ]] && break
        sleep 1
      done
      [[ "${gateway_health}" == *'"component":"gateway"'* ]]
    fi
  }

  send_message 1

  for _ in $(seq 1 30); do
    message_count=$(compose_exec mysql mysql -N -B -udipole -pdipole123 dipole \
      -e "SELECT COUNT(*) FROM messages WHERE content='${message_content}';")
    [[ "${message_count}" == "1" ]] && break
    sleep 1
  done
  test "${message_count}" = "1"

  if [[ -n "${message_restart_service}" ]]; then
    restart_message_service "${message_restart_service}"
  fi

  send_message 2

  for _ in $(seq 1 30); do
    message_count=$(compose_exec mysql mysql -N -B -udipole -pdipole123 dipole \
      -e "SELECT COUNT(*) FROM messages WHERE content='${message_content}';")
    outbox_count=$(compose_exec mysql mysql -N -B -udipole -pdipole123 dipole \
      -e "SELECT COUNT(*) FROM outbox_events WHERE value LIKE '%${message_content}%';")
    inbox_count=$(compose_exec mysql mysql -N -B -udipole -pdipole123 dipole \
      -e "SELECT COUNT(*) FROM user_sync_inbox i JOIN messages m ON m.uuid=i.message_uuid WHERE m.content='${message_content}' AND i.user_uuid='${target_uuid}';")
    [[ "${message_count}" == "1" && "${outbox_count}" == "1" && "${inbox_count}" == "1" ]] && break
    sleep 1
  done
  test "${message_count}" = "1"
  test "${outbox_count}" = "1"
  test "${inbox_count}" = "1"
  history_response=$(curl --fail --silent --show-error --connect-timeout 2 --max-time 5 \
    -H "Authorization: Bearer ${sender_token}" \
    "http://127.0.0.1:${gateway_port}/api/v1/messages/direct/${target_uuid}?before_seq=0&limit=20")
  jq -e --arg content "${message_content}" \
    'any(.data[]?; .content == $content and ((.message_seq // 0) > 0))' \
    <<<"${history_response}" >/dev/null
  incremental_response=$(curl --fail --silent --show-error --connect-timeout 2 --max-time 5 \
    -H "Authorization: Bearer ${target_token}" \
    "http://127.0.0.1:${gateway_port}/api/v1/messages/direct/${sender_uuid}?after_seq=0&limit=20")
  jq -e --arg content "${message_content}" \
    'any(.data[]?; .content == $content and ((.message_seq // 0) > 0))' \
    <<<"${incremental_response}" >/dev/null
  printf 'isolated candidate message flow passed: sender=%s target=%s restart_service=%s\n' \
    "${sender_uuid}" "${target_uuid}" "${message_restart_service:-none}"
fi

source_dirty=false
if [[ -n "$(git -C "${root_dir}" status --porcelain --untracked-files=no)" ]]; then
  source_dirty=true
fi
mkdir -p "$(dirname "${report_file}")"
jq -n \
  --arg schema "dipole.microservices.smoke-receipt.v1" \
  --arg project "${project}" \
  --arg revision "$(git -C "${root_dir}" rev-parse HEAD)" \
  --arg gateway_port "${gateway_port}" \
  --arg mode "${SMOKE_INBOX_PROJECTOR:-0}" \
  --arg message_flow "${SMOKE_MESSAGE_FLOW:-0}" \
  --arg message_restart_service "${message_restart_service}" \
  --arg message_count "${message_count:-0}" \
  --arg outbox_count "${outbox_count:-0}" \
  --arg inbox_count "${inbox_count:-0}" \
  --argjson source_dirty "${source_dirty}" \
  --arg rollback "remove deploy/microservices/inbox-projector.yml and restore DIPOLE_MESSAGE_INBOX_WRITE_MODE=atomic" \
  '{schema_version:$schema, compose_project:$project, source:{revision:$revision, dirty:$source_dirty}, validation:{gateway_port:($gateway_port|tonumber), inbox_projector:($mode == "1"), message_flow:($message_flow == "1"), message_recovery:{restart_service:$message_restart_service, message_count:($message_count|tonumber), outbox_count:($outbox_count|tonumber), inbox_count:($inbox_count|tonumber)}}, rollback:{action:$rollback, destructive_data_migration:false}}' \
  >"${report_file}"
chmod 600 "${report_file}"

printf 'isolated microservices smoke passed: project=%s gateway_port=%s search_profile=%s inbox_projector=%s\n' \
  "${project}" "${gateway_port}" "${SMOKE_SEARCH_PROFILE:-0}" "${SMOKE_INBOX_PROJECTOR:-0}"
printf 'isolated microservices receipt: %s\n' "${report_file}"
