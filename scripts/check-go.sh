#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

# Keep the canonical gate reproducible in a clean checkout while allowing an
# explicit caller-provided fixture for targeted configuration tests.
: "${DIPOLE_CONFIG_FILE:=$ROOT_DIR/configs/config.dist.yaml}"
export DIPOLE_CONFIG_FILE

packages=(./cmd/... ./db/... ./docs/swagger/... ./internal/...)

go test "${packages[@]}"
go vet "${packages[@]}"
