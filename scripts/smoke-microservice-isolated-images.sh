#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root_dir=$(cd "${script_dir}/.." && pwd)
project=${COMPOSE_PROJECT_NAME:-dipole-isolated-image-smoke}
gateway_port=${GATEWAY_PORT:-18080}
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

compose() {
  local -a profile_args=()
  if [[ "${SMOKE_SEARCH_PROFILE:-0}" == "1" ]]; then
    profile_args+=(--profile search)
  fi
  docker compose -p "${project}" \
    "${profile_args[@]}" \
    -f "${root_dir}/docker-compose.microservices.yml" \
    -f "${root_dir}/deploy/microservices/isolated-images.yml" \
    -f "${ports_file}" "$@"
}

cleanup() {
  local exit_code=$?
  if [[ "${KEEP_ON_FAILURE:-0}" != "1" || "${exit_code}" == "0" ]]; then
    compose down -v --remove-orphans >/dev/null 2>&1 || true
  else
    printf 'isolated microservices smoke retained failed project: %s\n' "${project}" >&2
  fi
  rm -f "${ports_file}"
  rm -f "${wscli_log}"
  rm -f "${wscli_binary}"
  if [[ "${remove_cert_dir}" == "1" ]]; then
    rm -rf "${cert_dir}"
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
export DIPOLE_IMAGE DIPOLE_MIGRATE_IMAGE DIPOLE_CORE_IMAGE DIPOLE_GATEWAY_IMAGE
export DIPOLE_MESSAGE_IMAGE DIPOLE_SYNC_IMAGE DIPOLE_SEARCH_IMAGE DIPOLE_SEARCH_INDEXER_IMAGE

INTERNAL_CERT_DIR="${cert_dir}" "${script_dir}/generate-internal-certs.sh" >/dev/null
compose config --quiet
compose up -d --wait
curl --fail --silent --show-error --connect-timeout 2 --max-time 5 "http://127.0.0.1:${gateway_port}/health" | grep -q '"component":"gateway"'

for service in core message sync gateway; do
  compose exec -T "${service}" wget -q -O - http://127.0.0.1:9100/livez | grep -qx alive
  compose exec -T "${service}" wget -q -O - http://127.0.0.1:9100/readyz | grep -qx ready
done

if [[ "${SMOKE_SEARCH_PROFILE:-0}" == "1" ]]; then
  for service in search search-indexer; do
    compose exec -T "${service}" wget -q -O - http://127.0.0.1:9100/livez | grep -qx alive
    compose exec -T "${service}" wget -q -O - http://127.0.0.1:9100/readyz | grep -qx ready
  done
fi

if [[ "${SMOKE_MESSAGE_FLOW:-0}" == "1" ]]; then
  (cd "${root_dir}" && CGO_ENABLED=0 go build -o "${wscli_binary}" ./cmd/tools/wscli)
  sender_phone="138$(printf '%08d' $(((RANDOM * 32768 + RANDOM) % 100000000)))"
  target_phone="139$(printf '%08d' $(((RANDOM * 32768 + RANDOM) % 100000000)))"
  sender_password=123456
  target_password=123456
  message_content="isolated-candidate-message-${project}"

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

  if { printf '/send %s\n' "${message_content}"; sleep 2; } |
    "${wscli_binary}" -base "http://127.0.0.1:${gateway_port}" \
      -telephone "${sender_phone}" -password "${sender_password}" -target "${target_uuid}" >"${wscli_log}" 2>&1; then
    wscli_status=0
  else
    wscli_status=$?
  fi
  if [[ "${wscli_status}" != "0" ]] && ! grep -q 'logged in as' "${wscli_log}"; then
    cat "${wscli_log}" >&2
    exit "${wscli_status}"
  fi

  for _ in $(seq 1 30); do
    message_count=$(compose exec -T mysql mysql -N -B -udipole -pdipole123 dipole \
      -e "SELECT COUNT(*) FROM messages WHERE content='${message_content}';")
    [[ "${message_count}" == "1" ]] && break
    sleep 1
  done
  test "${message_count}" = "1"
  outbox_count=$(compose exec -T mysql mysql -N -B -udipole -pdipole123 dipole \
    -e "SELECT COUNT(*) FROM outbox_events WHERE value LIKE '%${message_content}%';")
  inbox_count=$(compose exec -T mysql mysql -N -B -udipole -pdipole123 dipole \
    -e "SELECT COUNT(*) FROM user_sync_inbox i JOIN messages m ON m.uuid=i.message_uuid WHERE m.content='${message_content}' AND i.user_uuid='${target_uuid}';")
  test "${outbox_count}" = "1"
  test "${inbox_count}" = "1"
  printf 'isolated candidate message flow passed: sender=%s target=%s\n' "${sender_uuid}" "${target_uuid}"
fi

printf 'isolated microservices smoke passed: project=%s gateway_port=%s search_profile=%s\n' "${project}" "${gateway_port}" "${SMOKE_SEARCH_PROFILE:-0}"
