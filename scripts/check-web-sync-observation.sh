#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root_dir"

python3 -m unittest scripts.test_web_sync_observation
python3 -m py_compile scripts/web_sync_observation.py scripts/test_web_sync_observation.py
python3 -m json.tool contracts/web-sync-observation/v1/session.schema.json >/dev/null
python3 -m json.tool contracts/web-sync-observation/v1/evidence.schema.json >/dev/null
