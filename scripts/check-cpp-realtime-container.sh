#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
docker_bin="${DOCKER_BIN:-docker}"
image="${DIPOLE_CPP_CONTAINER_IMAGE:-dipole-realtime-delivery:gate}"

if ! command -v "${docker_bin}" >/dev/null 2>&1; then
  echo "Docker is required for the C++ container gate; set DOCKER_BIN when it is outside PATH" >&2
  exit 1
fi

revision="$(git -C "${root_dir}" rev-parse HEAD)"
created="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
if [[ -n "$(git -C "${root_dir}" status --porcelain)" ]]; then
  dirty=true
else
  dirty=false
fi

"${docker_bin}" build \
  --file "${root_dir}/services/realtime-delivery/Dockerfile" \
  --tag "${image}" \
  --build-arg "DIPOLE_VCS_REVISION=${revision}" \
  --build-arg "DIPOLE_BUILD_CREATED=${created}" \
  --build-arg "DIPOLE_VCS_DIRTY=${dirty}" \
  "${root_dir}"

printf 'C++ realtime container gate passed: image=%s revision=%s dirty=%s\n' "${image}" "${revision}" "${dirty}"
