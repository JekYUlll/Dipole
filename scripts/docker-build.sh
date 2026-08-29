#!/usr/bin/env bash
set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-dipole-server}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
COMPOSE_FILE="${COMPOSE_FILE:-deploy/compose/docker-compose.dist.yml}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BREW_BIN="/home/linuxbrew/.linuxbrew/bin"
export PATH="${BREW_BIN}:${PATH}"
NPM_BIN="${NPM_BIN:-$(command -v npm || true)}"
GO_BIN="${GO_BIN:-$(command -v go || true)}"

if [[ -z "${NPM_BIN}" && -x "${BREW_BIN}/npm" ]]; then
  NPM_BIN="${BREW_BIN}/npm"
fi

if [[ -z "${GO_BIN}" && -x "${BREW_BIN}/go" ]]; then
  GO_BIN="${BREW_BIN}/go"
fi

usage() {
  echo "Usage: $0 [build|up|deploy|down|restart|logs|frontend|backend]"
  echo ""
  echo "  frontend  Build frontend only (outputs to internal/server/webapp/)"
  echo "  backend   Build Go service binaries only (outputs to dist/)"
  echo "  build     Build frontend and Go service binaries locally, then package Docker image"
  echo "  up        Build image and start all services"
  echo "  deploy    Rebuild image and force-recreate dipole nodes (zero-downtime redeploy)"
  echo "  down      Stop and remove all containers"
  echo "  restart   Restart dipole nodes only (pick up config changes)"
  echo "  logs      Tail logs from dipole nodes"
  echo ""
  echo "Environment variables:"
  echo "  IMAGE_NAME   Image name (default: dipole-server)"
  echo "  IMAGE_TAG    Image tag  (default: latest)"
  echo "  COMPOSE_FILE Compose file (default: deploy/compose/docker-compose.dist.yml)"
  echo "  NODE_SERVICES Space-separated node services to deploy/restart/log"
  echo "  GO_BUILD_FLAGS Additional flags passed to go build"
  echo "  DIPOLE_BUILD_CREATED Override the embedded RFC3339 build time"
}

freeze_source_metadata() {
  DIPOLE_VCS_REVISION="$(git -C "${ROOT_DIR}" rev-parse HEAD)"
  DIPOLE_BUILD_CREATED="${DIPOLE_BUILD_CREATED:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
  if [[ -n "$(git -C "${ROOT_DIR}" status --porcelain)" ]]; then
    DIPOLE_VCS_DIRTY=true
  else
    DIPOLE_VCS_DIRTY=false
  fi
  export DIPOLE_VCS_REVISION DIPOLE_BUILD_CREATED DIPOLE_VCS_DIRTY
}

node_services() {
  if [[ -n "${NODE_SERVICES:-}" ]]; then
    echo "${NODE_SERVICES}"
    return
  fi

  docker compose -f "${ROOT_DIR}/${COMPOSE_FILE}" config --services | grep '^dipole-node' | tr '\n' ' ' | sed 's/[[:space:]]*$//'
}

cmd_frontend() {
  if [[ -z "${NPM_BIN}" ]]; then
    echo "npm not found; set NPM_BIN or install npm" >&2
    exit 1
  fi
  echo "==> Building frontend..."
  cd "${ROOT_DIR}/frontend"
  "${NPM_BIN}" ci --prefer-offline
  "${NPM_BIN}" run build
  echo "==> Frontend built → internal/server/webapp/"
}

cmd_backend() {
  if [[ -z "${GO_BIN}" ]]; then
    echo "go not found; set GO_BIN or install go" >&2
    exit 1
  fi
  echo "==> Building backend service binaries..."
  mkdir -p "${ROOT_DIR}/dist"
  (
    cd "${ROOT_DIR}"
    GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-server" ./cmd/services/core
    GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-gateway" ./cmd/services/gateway
    GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-message" ./cmd/services/message
    GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-migrate" ./cmd/tools/migrate
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-cassandra-projector" ./cmd/tools/cassandra-projector
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-search-indexer" ./cmd/services/search-indexer
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-search" ./cmd/services/search
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-sync" ./cmd/services/sync
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-sync-replay" ./cmd/tools/sync-replay
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-sync-reconcile" ./cmd/tools/sync-reconcile
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-sync-baseline" ./cmd/tools/sync-baseline
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-search-backfill" ./cmd/tools/search-backfill
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-search-reconcile" ./cmd/tools/search-reconcile
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-search-alias" ./cmd/tools/search-alias
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-search-archive" ./cmd/tools/search-archive
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-search-outbox-cleanup" ./cmd/tools/search-outbox-cleanup
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-cassandra-backfill" ./cmd/tools/cassandra-backfill
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-cassandra-reconcile" ./cmd/tools/cassandra-reconcile
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-cassandra-archive" ./cmd/tools/cassandra-archive
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-agent-artifact-reconcile" ./cmd/tools/agent-artifact-reconcile
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-agent-artifact-maintenance" ./cmd/tools/agent-artifact-maintenance
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-agent-task-timeline-repair" ./cmd/tools/agent-task-timeline-repair
	GOFLAGS=-mod=mod CGO_ENABLED=0 "${GO_BIN}" build ${GO_BUILD_FLAGS:-} -o "${ROOT_DIR}/dist/dipole-multipart-cleanup" ./cmd/tools/multipart-cleanup
  )
  echo "==> Backend built → dist/dipole-{server,gateway,message,search,sync,sync-replay,sync-reconcile,sync-baseline,migrate,cassandra-projector,search-indexer,search-backfill,search-reconcile,search-alias,search-archive,search-outbox-cleanup,cassandra-backfill,cassandra-reconcile,cassandra-archive,agent-artifact-reconcile,agent-artifact-maintenance,agent-task-timeline-repair,multipart-cleanup}"
}

cmd_build() {
  freeze_source_metadata
  cmd_frontend
  cmd_backend
  echo "==> Building Docker image ${IMAGE_NAME}:${IMAGE_TAG} (${DIPOLE_VCS_REVISION}, dirty=${DIPOLE_VCS_DIRTY})..."
  docker build \
    --build-arg DIPOLE_VCS_REVISION="${DIPOLE_VCS_REVISION}" \
    --build-arg DIPOLE_BUILD_CREATED="${DIPOLE_BUILD_CREATED}" \
    --build-arg DIPOLE_VCS_DIRTY="${DIPOLE_VCS_DIRTY}" \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    "${ROOT_DIR}"
  echo "==> Done: ${IMAGE_NAME}:${IMAGE_TAG}"
}

cmd_up() {
  cmd_build
  echo "==> Starting services with ${COMPOSE_FILE}..."
  docker compose -f "${ROOT_DIR}/${COMPOSE_FILE}" up -d
  echo "==> All services started."
}

cmd_deploy() {
  cmd_build
  local nodes
  nodes="$(node_services)"
  echo "==> Force-recreating dipole nodes: ${nodes}"
  docker compose -f "${ROOT_DIR}/${COMPOSE_FILE}" up -d --force-recreate ${nodes}
  echo "==> Nodes redeployed. Reloading nginx..."
  docker exec dipole-nginx nginx -s reload 2>/dev/null || true
  echo "==> Deploy complete."
}

cmd_down() {
  echo "==> Stopping services..."
  docker compose -f "${ROOT_DIR}/${COMPOSE_FILE}" down
}

cmd_restart() {
  local nodes
  nodes="$(node_services)"
  echo "==> Recreating dipole nodes with latest compose mounts: ${nodes}"
  docker compose -f "${ROOT_DIR}/${COMPOSE_FILE}" up -d --force-recreate ${nodes}
  echo "==> Nodes recreated."
}

cmd_logs() {
  local nodes
  nodes="$(node_services)"
  docker compose -f "${ROOT_DIR}/${COMPOSE_FILE}" logs -f ${nodes}
}

case "${1:-}" in
  frontend) cmd_frontend ;;
  backend)  cmd_backend ;;
  build)    cmd_build ;;
  up)       cmd_up ;;
  deploy)   cmd_deploy ;;
  down)     cmd_down ;;
  restart)  cmd_restart ;;
  logs)     cmd_logs ;;
  *)        usage; exit 1 ;;
esac
