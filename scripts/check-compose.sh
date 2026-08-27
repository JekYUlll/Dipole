#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

: "${DIPOLE_INTERNAL_RPC_SHARED_SECRET:=static-compose-validation-only}"
export DIPOLE_INTERNAL_RPC_SHARED_SECRET

for file in docker-compose*.yml; do
  docker compose -f "$file" config --quiet
done
