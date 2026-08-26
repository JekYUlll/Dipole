#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${repo_root}"

mapfile -t proto_files < <(find api/proto/dipole -name '*.proto' -type f | sort)
scripts/proto.sh "${proto_files[@]}"

generated_dir="internal/transport/grpc/gen"
if ! git diff --quiet -- "${generated_dir}"; then
  echo "protobuf generated files are stale; run scripts/check-proto.sh after updating protocol sources" >&2
  git diff -- "${generated_dir}" >&2
  exit 1
fi

if [[ -n "$(git ls-files --others --exclude-standard "${generated_dir}")" ]]; then
  echo "protobuf generated files are untracked; add the generated output" >&2
  exit 1
fi
