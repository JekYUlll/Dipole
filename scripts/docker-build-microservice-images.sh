#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "${root_dir}"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required" >&2
  exit 1
fi

if [[ ! -x dist/dipole-server ]]; then
  echo "Go binaries are missing; run scripts/docker-build.sh backend first" >&2
  exit 1
fi

revision=$(git rev-parse HEAD)
created=${DIPOLE_BUILD_CREATED:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}
dirty=false
if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then
  dirty=true
fi
context_dir="${root_dir}/dist"

declare -a services=(
  "migrate:dipole-migrate"
  "core:dipole-server"
  "gateway:dipole-gateway"
  "message:dipole-message"
  "sync:dipole-sync"
  "search:dipole-search"
  "search-indexer:dipole-search-indexer"
  "agent-timeline-repair:dipole-agent-task-timeline-repair"
)

for service_binary in "${services[@]}"; do
  service=${service_binary%%:*}
  binary=${service_binary#*:}
  case "${service}" in
    search-indexer) image_variable=DIPOLE_SEARCH_INDEXER_IMAGE ;;
    agent-timeline-repair) image_variable=DIPOLE_AGENT_TIMELINE_REPAIR_IMAGE ;;
    *) image_variable="DIPOLE_${service^^}_IMAGE" ;;
  esac
  image=${!image_variable:-dipole-${service}:latest}
  echo "==> building ${service} image ${image}"
  docker build \
    --file deploy/images/go-service.Dockerfile \
    --tag "${image}" \
    --build-arg "DIPOLE_BINARY=${binary}" \
    --build-arg "DIPOLE_VCS_REVISION=${revision}" \
    --build-arg "DIPOLE_BUILD_CREATED=${created}" \
    --build-arg "DIPOLE_BUILD_DIRTY=${dirty}" \
    "${context_dir}"
done

agent_image=${DIPOLE_AGENT_IMAGE:-dipole-agent:latest}
echo "==> building agent image ${agent_image}"
docker build \
  --file services/agent-runtime/Dockerfile \
  --tag "${agent_image}" \
  --build-arg "DIPOLE_VCS_REVISION=${revision}" \
  --build-arg "DIPOLE_BUILD_CREATED=${created}" \
  --build-arg "DIPOLE_VCS_DIRTY=${dirty}" \
  services/agent-runtime

printf 'microservice images built: revision=%s dirty=%s\n' "${revision}" "${dirty}"
