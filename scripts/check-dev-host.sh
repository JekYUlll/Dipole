#!/usr/bin/env bash
set -euo pipefail

# Validate a development host before any Compose project is started.
PROFILE="${DIPOLE_HOST_PROFILE:-${1:-}}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${DIPOLE_COMPOSE_FILE:-deploy/compose/docker-compose.microservices.yml}"

# Compose validation only; production/shared secrets must be supplied by the caller.
: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=static-compose-validation-only}"
export DIPOLE_INTERNAL_RPC_SHARED_SECRET

usage() {
  echo "Usage: $0 <remote-gpu|tencent-cloud|local>"
  echo "Optional overrides: DIPOLE_HOST_CPU, DIPOLE_HOST_MEMORY_MIB, DIPOLE_HOST_DISK_MIB"
  echo "Optional checks: DIPOLE_SKIP_DOCKER=1 DIPOLE_SKIP_COMPOSE=1"
}

case "${PROFILE}" in
  remote-gpu)
    required_cpu=8
    required_memory_mib=16384
    required_disk_mib=$((50 * 1024))
    ;;
  tencent-cloud)
    required_cpu=2
    required_memory_mib=1024
    required_disk_mib=$((10 * 1024))
    ;;
  local)
    required_cpu=4
    required_memory_mib=8192
    required_disk_mib=$((30 * 1024))
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac

host_cpu="${DIPOLE_HOST_CPU:-$(getconf _NPROCESSORS_ONLN)}"
host_memory_mib="${DIPOLE_HOST_MEMORY_MIB:-$(awk '/MemTotal:/ {print int($2 / 1024); exit}' /proc/meminfo)}"
host_disk_mib="${DIPOLE_HOST_DISK_MIB:-$(df -Pm "${ROOT_DIR}" | awk 'NR==2 {print $4; exit}')}"

failures=()
check_at_least() {
  local name="$1" actual="$2" required="$3"
  if ! [[ "${actual}" =~ ^[0-9]+$ ]] || (( actual < required )); then
    failures+=("${name}=${actual} required>=${required}")
  fi
}

check_at_least cpu "${host_cpu}" "${required_cpu}"
check_at_least memory_mib "${host_memory_mib}" "${required_memory_mib}"
check_at_least disk_mib "${host_disk_mib}" "${required_disk_mib}"

if [[ "${DIPOLE_SKIP_DOCKER:-0}" != "1" ]]; then
  command -v docker >/dev/null 2>&1 || failures+=("docker=missing")
  if command -v docker >/dev/null 2>&1 && ! docker info >/dev/null 2>&1; then
    failures+=("docker=daemon-unavailable")
  fi
fi

if [[ "${DIPOLE_SKIP_COMPOSE:-0}" != "1" ]]; then
  if ! command -v docker >/dev/null 2>&1; then
    failures+=("compose=docker-missing")
  elif [[ ! -f "${ROOT_DIR}/${COMPOSE_FILE}" ]]; then
    failures+=("compose-file=${COMPOSE_FILE}:missing")
  elif ! docker compose -f "${ROOT_DIR}/${COMPOSE_FILE}" config --quiet >/dev/null 2>&1; then
    failures+=("compose=config-invalid")
  fi
fi

printf 'Development host preflight: profile=%s cpu=%s memory_mib=%s disk_mib=%s compose=%s\n' \
  "${PROFILE}" "${host_cpu}" "${host_memory_mib}" "${host_disk_mib}" "${COMPOSE_FILE}"

if ((${#failures[@]} > 0)); then
  printf 'FAIL: %s\n' "${failures[*]}" >&2
  exit 1
fi

echo "PASS: host resources, Docker and Compose configuration are ready for ${PROFILE}"
