#!/usr/bin/env bash
set -euo pipefail

# Small-host read-only probe. Full throughput evidence still uses k6/run_bench.sh.
URL="${1:-${LOAD_URL:-}}"
REQUESTS="${REQUESTS:-100}"
CONCURRENCY="${CONCURRENCY:-4}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-5}"
EXPECTED_STATUS="${EXPECTED_STATUS:-200}"
REPORT_FILE="${REPORT_FILE:-}"

fail() {
  echo "http-read-load: $*" >&2
  exit 2
}

[[ -n "${URL}" ]] || fail "usage: $0 <http(s)://url>"
[[ "${URL}" =~ ^https?:// ]] || fail "URL must use http or https"
[[ "${REQUESTS}" =~ ^[1-9][0-9]*$ ]] || fail "REQUESTS must be a positive integer"
[[ "${CONCURRENCY}" =~ ^[1-9][0-9]*$ ]] || fail "CONCURRENCY must be a positive integer"
[[ "${TIMEOUT_SECONDS}" =~ ^[1-9][0-9]*$ ]] || fail "TIMEOUT_SECONDS must be a positive integer"
[[ "${EXPECTED_STATUS}" =~ ^[1-9][0-9]{2}$ ]] || fail "EXPECTED_STATUS must be an HTTP status"
(( CONCURRENCY <= REQUESTS )) || CONCURRENCY="${REQUESTS}"

for command in curl awk sort mktemp xargs; do
  command -v "${command}" >/dev/null 2>&1 || fail "required command not found: ${command}"
done

results_file="$(mktemp "${TMPDIR:-/tmp}/dipole-http-read-load.XXXXXX")"
cleanup() { rm -f "${results_file}"; }
trap cleanup EXIT

export LOAD_TARGET_URL="${URL}"
export LOAD_TIMEOUT_SECONDS="${TIMEOUT_SECONDS}"
seq "${REQUESTS}" | xargs -n1 -P"${CONCURRENCY}" bash -c '
  if result=$(curl --silent --show-error --output /dev/null \
      --connect-timeout "${LOAD_TIMEOUT_SECONDS}" --max-time "${LOAD_TIMEOUT_SECONDS}" \
      --write-out "%{http_code} %{time_total}" "${LOAD_TARGET_URL}" 2>/dev/null); then
    printf "%s\n" "${result}"
  else
    printf "000 0\n"
  fi
' _ >"${results_file}"

read -r total success failures p50 p95 p99 <<<"$(awk -v expected="${EXPECTED_STATUS}" '
  function percentile(rank, n, i) { i = int((rank * n + 99) / 100); if (i < 1) i = 1; if (i > n) i = n; return values[i] }
  { status=$1; time=$2 + 0; total++; if (status == expected) success++; else failures++; if (status != "000") { count++; values[count]=time } }
  END {
    if (count == 0) { printf "%d %d %d 0 0 0\n", total, success, failures; exit }
    for (i=1; i<=count; i++) for (j=i+1; j<=count; j++) if (values[j] < values[i]) { tmp=values[i]; values[i]=values[j]; values[j]=tmp }
    printf "%d %d %d %.6f %.6f %.6f\n", total, success, failures, percentile(50,count), percentile(95,count), percentile(99,count)
  }
' "${results_file}")"

timestamp="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
report="timestamp=${timestamp}
url=${URL}
method=GET
requests=${total}
success=${success}
failures=${failures}
expected_status=${EXPECTED_STATUS}
concurrency=${CONCURRENCY}
timeout_seconds=${TIMEOUT_SECONDS}
p50_seconds=${p50}
p95_seconds=${p95}
p99_seconds=${p99}"
printf '%s\n' "${report}"
if [[ -n "${REPORT_FILE}" ]]; then
  umask 077
  printf '%s\n' "${report}" >"${REPORT_FILE}"
fi
(( failures == 0 )) || exit 1
