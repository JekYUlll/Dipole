#!/usr/bin/env bash
set -euo pipefail

required_protoc_gen_go="v1.36.11"
required_protoc_gen_go_grpc="1.6.2"
protoc_gen_go="${PROTOC_GEN_GO:-protoc-gen-go}"
protoc_gen_go_grpc="${PROTOC_GEN_GO_GRPC:-protoc-gen-go-grpc}"

if ! command -v protoc >/dev/null 2>&1; then
  echo "protoc is required" >&2
  exit 1
fi
if ! command -v "${protoc_gen_go}" >/dev/null 2>&1; then
  echo "protoc-gen-go ${required_protoc_gen_go} is required" >&2
  exit 1
fi
if ! command -v "${protoc_gen_go_grpc}" >/dev/null 2>&1; then
  echo "protoc-gen-go-grpc ${required_protoc_gen_go_grpc} is required" >&2
  exit 1
fi

actual_go_version="$(${protoc_gen_go} --version | awk '{print $2}')"
actual_grpc_version="$(${protoc_gen_go_grpc} --version | awk '{print $2}')"
if [[ "${actual_go_version}" != "${required_protoc_gen_go}" ]]; then
  echo "protoc-gen-go version mismatch: expected ${required_protoc_gen_go}, got ${actual_go_version}" >&2
  exit 1
fi
if [[ "${actual_grpc_version}" != "${required_protoc_gen_go_grpc}" ]]; then
  echo "protoc-gen-go-grpc version mismatch: expected ${required_protoc_gen_go_grpc}, got ${actual_grpc_version}" >&2
  exit 1
fi

export PATH="$(dirname "$(command -v "${protoc_gen_go}")"):$(dirname "$(command -v "${protoc_gen_go_grpc}")"):${PATH}"

exec protoc \
  --proto_path=api/proto \
  --go_out=. \
  --go_opt=module=github.com/JekYUlll/Dipole \
  --go-grpc_out=. \
  --go-grpc_opt=module=github.com/JekYUlll/Dipole \
  "$@"
