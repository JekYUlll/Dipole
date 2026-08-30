#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
iterations="${DIPOLE_REALTIME_BENCH_ITERATIONS:-100000}"
minimum_ratio="${DIPOLE_REALTIME_BENCH_MIN_RATIO:-1.0}"
cpp_build_dir="${DIPOLE_CPP_BUILD_DIR:-/tmp/dipole-cpp-realtime-benchmark}"
cpp_binary="${cpp_build_dir}/dipole-realtime-projection-benchmark"
output_file="${DIPOLE_REALTIME_BENCH_OUTPUT:-${root_dir}/benchmarks/c2-cpp-projection-benchmark-$(date +%Y-%m-%d)/report.json}"
cpp_runner="host"
compiler_label="${CXX:-/usr/bin/g++}"

if [[ "${DIPOLE_REALTIME_BENCH_CONTAINER:-0}" == "1" ]]; then
  docker_bin="${DOCKER_BIN:-docker}"
  image="${DIPOLE_REALTIME_BENCH_IMAGE:-dipole-realtime-delivery:benchmark-$(git -C "${root_dir}" rev-parse --short HEAD)}"
  command -v "${docker_bin}" >/dev/null 2>&1 || {
    echo "Docker is required when DIPOLE_REALTIME_BENCH_CONTAINER=1" >&2
    exit 1
  }
  revision="$(git -C "${root_dir}" rev-parse HEAD)"
  created="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  dirty=false
  [[ -n "$(git -C "${root_dir}" status --porcelain)" ]] && dirty=true
  "${docker_bin}" build \
    --target builder \
    --file "${root_dir}/services/realtime-delivery/Dockerfile" \
    --tag "${image}" \
    --build-arg "DIPOLE_VCS_REVISION=${revision}" \
    --build-arg "DIPOLE_BUILD_CREATED=${created}" \
    --build-arg "DIPOLE_VCS_DIRTY=${dirty}" \
    "${root_dir}"
  cpp_runner="container"
  compiler_label="container:g++"
  cpp_json="$("${docker_bin}" run --rm "${image}" /build/dipole-realtime-projection-benchmark "${iterations}")"
elif [[ ! -x "${cpp_binary}" ]]; then
  CXX="${CXX:-/usr/bin/g++}" \
    DIPOLE_GRPC_ROOT="${DIPOLE_GRPC_ROOT:-}" \
    DIPOLE_RDKAFKA_ROOT="${DIPOLE_RDKAFKA_ROOT:-}" \
    DIPOLE_CPP_BUILD_DIR="${cpp_build_dir}" \
    "${root_dir}/scripts/check-cpp-realtime.sh"
  cpp_json="$(LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu "${cpp_binary}" "${iterations}")"
else
  cpp_json="$(LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu "${cpp_binary}" "${iterations}")"
fi
go_json="$(cd "${root_dir}" && LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}} go run ./scripts/bench/realtime_projection_benchmark.go "${iterations}")"

mkdir -p "$(dirname "${output_file}")"
revision="$(git -C "${root_dir}" rev-parse HEAD)"
python3 - "${output_file}" "${cpp_json}" "${go_json}" "${minimum_ratio}" "${revision}" "${compiler_label}" "${cpp_runner}" <<'PY'
import json
import pathlib
import sys

output, cpp_raw, go_raw, minimum_ratio, revision, compiler, runner = sys.argv[1:]
cpp = json.loads(cpp_raw)
go = json.loads(go_raw)
minimum_ratio = float(minimum_ratio)
if cpp["schema_version"] != go["schema_version"]:
    raise SystemExit("benchmark schema mismatch")
if cpp["iterations"] != go["iterations"]:
    raise SystemExit("benchmark iteration mismatch")
report = {
    "schema_version": "dipole.realtime.projection-benchmark-report.v1",
    "git_revision": revision,
    "compiler": compiler,
    "cpp_runner": runner,
    "workload": {"iterations": cpp["iterations"], "event": "message.direct.created"},
    "go": go,
    "cpp": cpp,
    "cpp_over_go_ops_ratio": cpp["ops_per_second"] / go["ops_per_second"] if go["ops_per_second"] else 0,
    "minimum_cpp_over_go_ops_ratio": minimum_ratio,
    "decision": "eligible" if cpp["item_count"] == go["item_count"] and cpp["item_count"] > 0 and cpp["ops_per_second"] / go["ops_per_second"] >= minimum_ratio else "blocked",
}
pathlib.Path(output).write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
print(json.dumps({"decision": report["decision"], "cpp_over_go_ops_ratio": report["cpp_over_go_ops_ratio"]}))
PY
