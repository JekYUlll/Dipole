#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

packages=(./cmd/... ./db/... ./docs/swagger/... ./internal/...)

go test "${packages[@]}"
go vet "${packages[@]}"
