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

DIPOLE_REALTIME_DELIVERY=go \
DIPOLE_REALTIME_KAFKA_BROKERS=127.0.0.1:1 \
DIPOLE_REALTIME_EVIDENCE_FILE="${evidence}" \
  "${binary}" shadow "${testdata}" >"${log_file}" 2>&1 && {
  echo "shadow runtime unexpectedly accepted Go authority" >&2
  exit 1
}

DIPOLE_REALTIME_DELIVERY=shadow \
DIPOLE_REALTIME_HOST=127.0.0.1 \
DIPOLE_REALTIME_PORT="${port}" \
DIPOLE_REALTIME_KAFKA_BROKERS=127.0.0.1:1 \
DIPOLE_REALTIME_KAFKA_GROUP_ID=dipole-realtime-shadow-health-test \
DIPOLE_REALTIME_EVIDENCE_FILE="${evidence}" \
  "${binary}" shadow "${testdata}" >"${log_file}" 2>&1 &
pid="$!"

for _ in $(seq 1 50); do
  if curl --fail --silent "http://127.0.0.1:${port}/livez" | grep -q '"mode":"shadow"'; then
    break
  fi
  sleep 0.05
done

curl --fail --silent "http://127.0.0.1:${port}/livez" | grep -q '"status":"ok"'
status="$(curl --silent --output /tmp/dipole-shadow-health-body-"${pid}" --write-out '%{http_code}' "http://127.0.0.1:${port}/readyz")"
test "${status}" = "503"
grep -q '"status":"not_ready"' /tmp/dipole-shadow-health-body-"${pid}"
rm -f /tmp/dipole-shadow-health-body-"${pid}"

kill -TERM "${pid}"
wait "${pid}"
pid=""
