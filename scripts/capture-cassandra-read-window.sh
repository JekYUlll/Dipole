#!/usr/bin/env bash
set -euo pipefail

# Capture an immutable pair of Message Service metric snapshots for read-rollout evidence.
: "${DIPOLE_CASSANDRA_READ_METRICS_URL:?set the Message Service /metrics URL}"
: "${DIPOLE_CASSANDRA_READ_WINDOW_DIR:?set an empty output directory}"
phase=${1:-}
if [[ "${phase}" != start && "${phase}" != end ]]; then
  printf 'usage: %s start|end\n' "$0" >&2
  exit 1
fi

umask 077
capture() { curl --fail --silent --show-error --max-time 10 "${DIPOLE_CASSANDRA_READ_METRICS_URL}"; }
if [[ "${phase}" == start ]]; then
  : "${DIPOLE_CASSANDRA_READ_DEPLOYMENT_REVISION:?set the deployed Message revision}"
  : "${DIPOLE_CASSANDRA_READ_PERCENTAGE:?set the configured Cassandra read percentage}"
  [[ "${DIPOLE_CASSANDRA_READ_PERCENTAGE}" =~ ^([0-9]|[1-9][0-9]|100)$ ]] || { echo 'invalid read percentage' >&2; exit 1; }
  [[ ! -e "${DIPOLE_CASSANDRA_READ_WINDOW_DIR}" || -z "$(find "${DIPOLE_CASSANDRA_READ_WINDOW_DIR}" -mindepth 1 -print -quit)" ]] || { echo 'refusing to overwrite evidence directory' >&2; exit 1; }
  mkdir -p "${DIPOLE_CASSANDRA_READ_WINDOW_DIR}"
  capture >"${DIPOLE_CASSANDRA_READ_WINDOW_DIR}/metrics-start.prom"
  printf '{"deploymentRevision":"%s","configuredReadPercentage":%s,"windowStart":"%s"}\n' "${DIPOLE_CASSANDRA_READ_DEPLOYMENT_REVISION}" "${DIPOLE_CASSANDRA_READ_PERCENTAGE}" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >"${DIPOLE_CASSANDRA_READ_WINDOW_DIR}/metadata.json"
else
  [[ -f "${DIPOLE_CASSANDRA_READ_WINDOW_DIR}/metadata.json" && ! -e "${DIPOLE_CASSANDRA_READ_WINDOW_DIR}/metrics-end.prom" ]] || { echo 'missing start metadata or end snapshot already exists' >&2; exit 1; }
  capture >"${DIPOLE_CASSANDRA_READ_WINDOW_DIR}/metrics-end.prom"
  date -u +%Y-%m-%dT%H:%M:%SZ >"${DIPOLE_CASSANDRA_READ_WINDOW_DIR}/window-end.txt"
fi
