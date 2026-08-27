#!/usr/bin/env bash
set -euo pipefail

binary="$1"
testdata="$2"
port="$(shuf -i 20000-45000 -n 1)"
evidence="$(mktemp)"
log_file="$(mktemp)"

pid=""
cleanup() {
  if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
    kill -TERM "${pid}"
    wait "${pid}" || true
  fi
  rm -f "${evidence}" "${log_file}"
}
trap cleanup EXIT

if DIPOLE_REALTIME_KAFKA_BROKERS=127.0.0.1:1 \
  DIPOLE_REALTIME_EVIDENCE_FILE="${evidence}" \
  "${binary}" primary "${testdata}" >/dev/null 2>&1; then
  echo "primary runtime unexpectedly started without its enable gate" >&2
  exit 1
fi

DIPOLE_REALTIME_PRIMARY_ENABLED=true \
DIPOLE_REALTIME_HOST=127.0.0.1 \
DIPOLE_REALTIME_PORT="${port}" \
DIPOLE_REALTIME_KAFKA_BROKERS=127.0.0.1:1 \
DIPOLE_REALTIME_KAFKA_GROUP_ID=dipole-realtime-primary-health-test \
DIPOLE_REALTIME_EVIDENCE_FILE="${evidence}" \
DIPOLE_REALTIME_PRESENCE_MODE=primary \
DIPOLE_REALTIME_REDIS_ENDPOINT=127.0.0.1:1 \
DIPOLE_REALTIME_NODE_TRANSPORT_MODE=primary \
DIPOLE_REALTIME_NODE_TARGETS=node-a=127.0.0.1:1 \
DIPOLE_INTERNAL_RPC_SHARED_SECRET=primary-health-secret \
  "${binary}" primary "${testdata}" >"${log_file}" 2>&1 &
pid="$!"

for _ in $(seq 1 50); do
  if curl --fail --silent "http://127.0.0.1:${port}/livez" | grep -q '"mode":"primary"'; then
    break
  fi
  sleep 0.05
done

curl --fail --silent "http://127.0.0.1:${port}/livez" | grep -q '"status":"ok"'
body="$(mktemp)"
status="$(curl --silent --output "${body}" --write-out '%{http_code}' \
  "http://127.0.0.1:${port}/readyz")"
test "${status}" = "503"
grep -q '"status":"not_ready"' "${body}"
rm -f "${body}"

kill -TERM "${pid}"
wait "${pid}"
pid=""
