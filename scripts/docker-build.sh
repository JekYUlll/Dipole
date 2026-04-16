#!/usr/bin/env bash
set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-dipole-server}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.dist.yml}"

usage() {
  echo "Usage: $0 [build|up|down|restart|logs]"
  echo ""
  echo "  build    Build the Docker image (multi-stage, no local Go required)"
  echo "  up       Build image and start all services"
  echo "  down     Stop and remove all containers"
  echo "  restart  Restart dipole nodes only (pick up config changes)"
  echo "  logs     Tail logs from dipole nodes"
  echo ""
  echo "Environment variables:"
  echo "  IMAGE_NAME   Image name (default: dipole-server)"
  echo "  IMAGE_TAG    Image tag  (default: latest)"
  echo "  COMPOSE_FILE Compose file (default: docker-compose.dist.yml)"
}

cmd_build() {
  echo "==> Building Docker image ${IMAGE_NAME}:${IMAGE_TAG}..."
  docker build -t "${IMAGE_NAME}:${IMAGE_TAG}" .
  echo "==> Done: ${IMAGE_NAME}:${IMAGE_TAG}"
}

cmd_up() {
  cmd_build
  echo "==> Starting services with ${COMPOSE_FILE}..."
  docker compose -f "${COMPOSE_FILE}" up -d
  echo "==> All services started."
}

cmd_down() {
  echo "==> Stopping services..."
  docker compose -f "${COMPOSE_FILE}" down
}

cmd_restart() {
  echo "==> Restarting dipole nodes..."
  docker compose -f "${COMPOSE_FILE}" restart dipole-node1 dipole-node2
  echo "==> Nodes restarted."
}

cmd_logs() {
  docker compose -f "${COMPOSE_FILE}" logs -f dipole-node1 dipole-node2
}

case "${1:-}" in
  build)   cmd_build ;;
  up)      cmd_up ;;
  down)    cmd_down ;;
  restart) cmd_restart ;;
  logs)    cmd_logs ;;
  *)       usage; exit 1 ;;
esac
