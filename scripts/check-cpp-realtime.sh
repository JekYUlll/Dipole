#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cxx_compiler="${CXX:-/usr/bin/g++}"
compiler_id="$(basename "${cxx_compiler}" | tr -cd '[:alnum:]_.+-')"
build_dir="${DIPOLE_CPP_BUILD_DIR:-/tmp/dipole-cpp-realtime-build-${compiler_id}}"
clang_tidy="${CLANG_TIDY_BIN:-$(command -v clang-tidy || true)}"

if [[ -z "${clang_tidy}" ]]; then
  echo "clang-tidy is required; set CLANG_TIDY_BIN when it is outside PATH" >&2
  exit 1
fi

cmake \
  -S "${root_dir}/realtime-delivery" \
  -B "${build_dir}" \
  -G Ninja \
  -DCMAKE_CXX_COMPILER="${cxx_compiler}" \
  -DCMAKE_BUILD_TYPE=RelWithDebInfo \
  -DBUILD_TESTING=ON
cmake --build "${build_dir}" --parallel
"${clang_tidy}" \
  --checks='bugprone-*,performance-*,portability-*' \
  --warnings-as-errors='*' \
  --exclude-header-filter='.*generated.*' \
  -p "${build_dir}" \
  "${root_dir}/realtime-delivery/src/contract_validator.cpp" \
  "${root_dir}/realtime-delivery/src/event_projection.cpp" \
  "${root_dir}/realtime-delivery/src/health_server.cpp" \
  "${root_dir}/realtime-delivery/src/main.cpp" \
  "${root_dir}/realtime-delivery/tests/contract_test.cpp" \
  "${root_dir}/realtime-delivery/tests/event_projection_test.cpp"
ctest --test-dir "${build_dir}" --output-on-failure
