#!/usr/bin/env bash
set -euo pipefail

IMAGE_NAME="${IMAGE_NAME:-dipole-server}"
IMAGE_TAG="${IMAGE_TAG:-latest}"

echo "==> Building binary for linux/amd64..."
mkdir -p dist
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/dipole-server ./cmd/server

echo "==> Building Docker image ${IMAGE_NAME}:${IMAGE_TAG}..."
docker build -t "${IMAGE_NAME}:${IMAGE_TAG}" .

echo "==> Done: ${IMAGE_NAME}:${IMAGE_TAG}"
