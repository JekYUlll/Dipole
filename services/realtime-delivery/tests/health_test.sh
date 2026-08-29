#!/usr/bin/env bash
set -euo pipefail

binary="$1"
testdata="$2"
port="$(shuf -i 20000-45000 -n 1)"
log_file="$(mktemp)"

pid=""
cleanup() {
  if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
    kill -TERM "${pid}"
    wait "${pid}" || true
  fi
  rm -f "${log_file}"
}
trap cleanup EXIT

DIPOLE_REALTIME_HOST=127.0.0.1 \
DIPOLE_REALTIME_PORT="${port}" \
DIPOLE_REALTIME_MODE=contract_only \
  "${binary}" serve "${testdata}" >"${log_file}" 2>&1 &
pid="$!"

for _ in $(seq 1 50); do
  if curl --fail --silent "http://127.0.0.1:${port}/readyz" | grep -q '"service":"dipole-realtime-delivery"'; then
    break
  fi
  sleep 0.05
done

curl --fail --silent "http://127.0.0.1:${port}/livez" | grep -q '"mode":"contract_only"'
curl --fail --silent "http://127.0.0.1:${port}/health" | grep -q '"status":"ok"'
if curl --fail --silent "http://127.0.0.1:${port}/missing" >/dev/null; then
  echo "unknown health path unexpectedly succeeded" >&2
  exit 1
fi

kill -TERM "${pid}"
wait "${pid}"
pid=""

if DIPOLE_REALTIME_PORT=0 "${binary}" serve "${testdata}" >/dev/null 2>&1; then
  echo "invalid port unexpectedly succeeded" >&2
  exit 1
fi
if DIPOLE_REALTIME_MODE=shadow "${binary}" serve "${testdata}" >/dev/null 2>&1; then
  echo "shadow mode unexpectedly enabled" >&2
  exit 1
fi
