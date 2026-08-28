#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
iterations="${DIPOLE_REALTIME_BENCH_ITERATIONS:-100000}"
minimum_ratio="${DIPOLE_REALTIME_BENCH_MIN_RATIO:-1.0}"
cpp_build_dir="${DIPOLE_CPP_BUILD_DIR:-/tmp/dipole-cpp-realtime-benchmark}"
cpp_binary="${cpp_build_dir}/dipole-realtime-projection-benchmark"
output_file="${DIPOLE_REALTIME_BENCH_OUTPUT:-${root_dir}/benchmarks/c2-cpp-projection-benchmark-$(date +%Y-%m-%d)/report.json}"

if [[ ! -x "${cpp_binary}" ]]; then
  CXX="${CXX:-/usr/bin/g++}" \
    DIPOLE_GRPC_ROOT="${DIPOLE_GRPC_ROOT:-}" \
    DIPOLE_RDKAFKA_ROOT="${DIPOLE_RDKAFKA_ROOT:-}" \
    DIPOLE_CPP_BUILD_DIR="${cpp_build_dir}" \
    "${root_dir}/scripts/check-cpp-realtime.sh"
fi

cpp_json="$(LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu "${cpp_binary}" "${iterations}")"
go_json="$(cd "${root_dir}" && LD_LIBRARY_PATH=/usr/lib/x86_64-linux-gnu${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}} go run ./scripts/bench/realtime_projection_benchmark.go "${iterations}")"

mkdir -p "$(dirname "${output_file}")"
revision="$(git -C "${root_dir}" rev-parse HEAD)"
compiler="${CXX:-/usr/bin/g++}"
python3 - "${output_file}" "${cpp_json}" "${go_json}" "${minimum_ratio}" "${revision}" "${compiler}" <<'PY'
import json
import pathlib
import sys

output, cpp_raw, go_raw, minimum_ratio, revision, compiler = sys.argv[1:]
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
