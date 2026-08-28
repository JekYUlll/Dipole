#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cxx_compiler="${CXX:-/usr/bin/g++}"
compiler_id="$(basename "${cxx_compiler}" | tr -cd '[:alnum:]_.+-')"
compiler_path="${DIPOLE_CPP_COMPILER_PATH:-$(dirname "${cxx_compiler}")}"
build_dir="${DIPOLE_CPP_BUILD_DIR:-/tmp/dipole-cpp-realtime-build-${compiler_id}}"
clang_tidy="${CLANG_TIDY_BIN:-$(command -v clang-tidy || true)}"
rdkafka_root="${DIPOLE_RDKAFKA_ROOT:-}"
grpc_root="${DIPOLE_GRPC_ROOT:-}"

if [[ -z "${clang_tidy}" ]]; then
  echo "clang-tidy is required; set CLANG_TIDY_BIN when it is outside PATH" >&2
  exit 1
fi

export COMPILER_PATH="${compiler_path}"

cmake \
  -S "${root_dir}/realtime-delivery" \
  -B "${build_dir}" \
  -G Ninja \
  -DCMAKE_CXX_COMPILER="${cxx_compiler}" \
  -DCMAKE_BUILD_TYPE=RelWithDebInfo \
  -DDIPOLE_RDKAFKA_ROOT="${rdkafka_root}" \
  -DDIPOLE_GRPC_ROOT="${grpc_root}" \
  -DBUILD_TESTING=ON
cmake --build "${build_dir}" --parallel
"${clang_tidy}" \
  --checks='bugprone-*,performance-*,portability-*' \
  --warnings-as-errors='*' \
  --header-filter="${root_dir}/realtime-delivery/(src|tests)/.*" \
  --exclude-header-filter='.*generated.*' \
  -p "${build_dir}" \
  "${root_dir}/realtime-delivery/src/authority_fence.cpp" \
  "${root_dir}/realtime-delivery/src/contract_validator.cpp" \
  "${root_dir}/realtime-delivery/src/event_projection.cpp" \
  "${root_dir}/realtime-delivery/src/health_server.cpp" \
  "${root_dir}/realtime-delivery/src/hiredis_presence_reader.cpp" \
  "${root_dir}/realtime-delivery/src/librdkafka_consumer.cpp" \
  "${root_dir}/realtime-delivery/src/node_delivery_transport.cpp" \
  "${root_dir}/realtime-delivery/src/main.cpp" \
  "${root_dir}/realtime-delivery/src/presence_projection.cpp" \
  "${root_dir}/realtime-delivery/src/primary_probe.cpp" \
  "${root_dir}/realtime-delivery/src/shadow_evidence.cpp" \
  "${root_dir}/realtime-delivery/src/shadow_runner.cpp" \
  "${root_dir}/realtime-delivery/src/shadow_runtime.cpp" \
  "${root_dir}/realtime-delivery/tests/authority_fence_test.cpp" \
  "${root_dir}/realtime-delivery/tests/contract_test.cpp" \
  "${root_dir}/realtime-delivery/tests/event_projection_test.cpp" \
  "${root_dir}/realtime-delivery/tests/hiredis_presence_reader_test.cpp" \
  "${root_dir}/realtime-delivery/tests/librdkafka_consumer_test.cpp" \
  "${root_dir}/realtime-delivery/tests/node_delivery_transport_test.cpp" \
  "${root_dir}/realtime-delivery/tests/presence_projection_test.cpp" \
  "${root_dir}/realtime-delivery/tests/primary_probe_test.cpp" \
  "${root_dir}/realtime-delivery/tests/shadow_evidence_test.cpp" \
  "${root_dir}/realtime-delivery/tests/shadow_runner_test.cpp"
if [[ -n "${rdkafka_root}" ]]; then
  export LD_LIBRARY_PATH="${rdkafka_root}/usr/lib/x86_64-linux-gnu${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}}"
fi
ctest --test-dir "${build_dir}" --output-on-failure
PYTHONPATH="${root_dir}/scripts" python3 -m unittest \
  "${root_dir}/scripts/test_realtime_delivery_comparison.py"
python3 -m json.tool \
  "${root_dir}/contracts/realtime-delivery-comparison/v1/report.schema.json" >/dev/null
