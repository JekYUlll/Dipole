#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repo_root}"

scripts/proto.sh api/proto/dipole/message/v1/message.proto

generated_dir="internal/transport/grpc/gen"
if ! git diff --quiet -- "${generated_dir}"; then
  echo "protobuf generated files are stale; run scripts/proto.sh api/proto/dipole/message/v1/message.proto" >&2
  git diff -- "${generated_dir}" >&2
  exit 1
fi

if [[ -n "$(git ls-files --others --exclude-standard "${generated_dir}")" ]]; then
  echo "protobuf generated files are untracked; add the generated output" >&2
  exit 1
fi
