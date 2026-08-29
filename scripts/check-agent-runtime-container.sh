#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
docker_bin="${DOCKER_BIN:-docker}"
image="${DIPOLE_AGENT_CONTAINER_IMAGE:-dipole-agent-runtime:gate}"
container="dipole-agent-runtime-gate-$$"

if ! command -v "${docker_bin}" >/dev/null 2>&1; then
  echo "Docker is required for the Agent Runtime container gate; set DOCKER_BIN when it is outside PATH" >&2
  exit 1
fi

revision="$(git -C "${root_dir}" rev-parse HEAD)"
created="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
if [[ -n "$(git -C "${root_dir}" status --porcelain --untracked-files=no)" ]]; then
  dirty=true
else
  dirty=false
fi

cleanup() {
  "${docker_bin}" rm -f "${container}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

"${docker_bin}" build \
  --file "${root_dir}/services/agent-runtime/Dockerfile" \
  --tag "${image}" \
  --build-arg "DIPOLE_VCS_REVISION=${revision}" \
  --build-arg "DIPOLE_BUILD_CREATED=${created}" \
  --build-arg "DIPOLE_VCS_DIRTY=${dirty}" \
  "${root_dir}/services/agent-runtime"

actual_revision="$("${docker_bin}" image inspect --format '{{index .Config.Labels "org.opencontainers.image.revision"}}' "${image}")"
actual_dirty="$("${docker_bin}" image inspect --format '{{index .Config.Labels "io.dipole.source.dirty"}}' "${image}")"
if [[ "${actual_revision}" != "${revision}" || "${actual_dirty}" != "${dirty}" ]]; then
  echo "Agent Runtime image provenance mismatch: revision=${actual_revision} dirty=${actual_dirty}" >&2
  exit 1
fi

"${docker_bin}" run --detach --name "${container}" \
  -e DIPOLE_AGENT_KAFKA_ENABLED=false \
  -e DIPOLE_AGENT_CAPABILITY_RPC_ENABLED=false \
  "${image}" >/dev/null

for _ in {1..30}; do
  if "${docker_bin}" exec "${container}" node -e "fetch('http://127.0.0.1:8091/readyz').then(async r=>{if(!r.ok){process.exit(1)}; console.log(await r.text())}).catch(()=>process.exit(1))" >/dev/null; then
    user="$("${docker_bin}" inspect --format '{{.Config.User}}' "${container}")"
    if [[ "${user}" != "node" ]]; then
      echo "Agent Runtime container must run as node, got ${user}" >&2
      exit 1
    fi
    printf 'Agent Runtime container gate passed: image=%s revision=%s dirty=%s user=%s\n' "${image}" "${revision}" "${dirty}" "${user}"
    exit 0
  fi
  sleep 1
done

echo "Agent Runtime container did not become ready" >&2
"${docker_bin}" logs "${container}" >&2 || true
exit 1
