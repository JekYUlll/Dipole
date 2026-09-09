#!/usr/bin/env bash
set -euo pipefail

# Applies the narrow Gateway-only promotion maintenance overlay. The operator
# still performs proposal/review separately; this tool owns the reversible
# Compose transition so an ad-hoc command cannot omit the certificate mount.

usage() {
  cat <<'EOF'
Usage:
  run-agent-promotion-window.sh open|status|close --config PATH [--apply]

The JSON config contains only deployment locations and non-secret identifiers:
{
  "compose_project": "dipole-experience",
  "env_file": "/absolute/path/to/.env",
  "compose_files": ["/absolute/path/to/base.yml", "/absolute/path/to/agent.yml"],
  "promotion_overlay": "/absolute/path/to/agent-promotion-experience.yml",
  "certificate_dir": "/absolute/path/to/internal/certs",
  "gateway_service": "gateway",
  "tenant_id": "dipole"
}

Without --apply the tool prints the exact target and performs no Docker action.
`open` renders the reviewed base plus maintenance overlay and recreates only the
Gateway. `close` renders the base files only and restores the default-disabled
route. `status` verifies container health and the effective route environment.
The config must not contain credentials; use docker compose --env-file for
deployment secrets and never source that file.
EOF
}

die() {
  printf '%s\n' "$*" >&2
  exit 2
}

action=${1:-}
[[ "$action" == "open" || "$action" == "status" || "$action" == "close" ]] || { usage >&2; die "first argument must be open, status, or close"; }
shift

config_file=
apply=0
while (( $# > 0 )); do
  case "$1" in
    --config) config_file=${2:-}; shift 2 ;;
    --apply) apply=1; shift ;;
    --help|-h) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

[[ -n "$config_file" && -r "$config_file" ]] || die "--config must name a readable JSON file"
command -v python3 >/dev/null 2>&1 || die "python3 is required to validate the config"

config_values=$(mktemp)
trap 'rm -f "$config_values"' EXIT
if ! python3 - "$config_file" >"$config_values" <<'PY'
import json
import os
import re
import sys

path = os.path.abspath(sys.argv[1])
try:
    with open(path, encoding="utf-8") as handle:
        config = json.load(handle)
except (OSError, json.JSONDecodeError) as exc:
    raise SystemExit(f"invalid promotion window config: {exc}")

required = {
    "compose_project": str,
    "env_file": str,
    "compose_files": list,
    "promotion_overlay": str,
    "certificate_dir": str,
    "gateway_service": str,
    "tenant_id": str,
}
if set(config) != set(required):
    raise SystemExit("config must contain exactly: " + ", ".join(required))
for key, expected in required.items():
    if not isinstance(config[key], expected):
        raise SystemExit(f"config {key} has the wrong type")
for key in ("env_file", "promotion_overlay", "certificate_dir"):
    if not config[key].startswith("/"):
        raise SystemExit(f"config {key} must be absolute")
if not config["compose_files"]:
    raise SystemExit("config compose_files must not be empty")
for compose_file in config["compose_files"]:
    if not isinstance(compose_file, str) or not compose_file.startswith("/"):
        raise SystemExit("every compose_files entry must be an absolute path")
for key in ("compose_project", "gateway_service", "tenant_id"):
    value = config[key]
    if not re.fullmatch(r"[A-Za-z0-9._:-]{1,96}", value):
        raise SystemExit(f"invalid config {key}")
for value in [config["env_file"], config["promotion_overlay"], config["certificate_dir"], *config["compose_files"]]:
    if any(character in value for character in "\n\r\t"):
        raise SystemExit("config paths must not contain control characters")
for key in ("env_file", "promotion_overlay", "certificate_dir"):
    if not os.path.exists(config[key]):
        raise SystemExit(f"config {key} does not exist: {config[key]}")
if not os.path.isdir(config["certificate_dir"]):
    raise SystemExit("config certificate_dir must be a directory")
for compose_file in config["compose_files"]:
    if not os.path.isfile(compose_file):
        raise SystemExit(f"compose file does not exist: {compose_file}")
for value in (config["compose_project"], config["env_file"], config["promotion_overlay"], config["certificate_dir"], config["gateway_service"], config["tenant_id"], *config["compose_files"]):
    print(value)
PY
then
  die "promotion window config validation failed"
fi

mapfile -t values <"$config_values"
compose_project=${values[0]}
env_file=${values[1]}
promotion_overlay=${values[2]}
certificate_dir=${values[3]}
gateway_service=${values[4]}
tenant_id=${values[5]}
compose_files=("${values[@]:6}")

compose=(docker compose --env-file "$env_file" -p "$compose_project")
for compose_file in "${compose_files[@]}"; do
  compose+=(-f "$compose_file")
done
overlay_compose=("${compose[@]}" -f "$promotion_overlay")

print_plan() {
  printf 'Promotion window %s plan: project=%s gateway=%s tenant=%s\n' "$action" "$compose_project" "$gateway_service" "$tenant_id"
  printf 'certificate_dir=%s\n' "$certificate_dir"
  printf 'base_compose_files=%s\n' "${compose_files[*]}"
  [[ "$action" != "open" ]] || printf 'promotion_overlay=%s\n' "$promotion_overlay"
}

container_id() {
  "${compose[@]}" ps -q "$gateway_service"
}

assert_gateway_state() {
  local expected=$1 id health route
  # A recreate reports "starting" briefly even when Compose has succeeded.
  # Poll explicitly so close has the same bounded health contract as open.
  for _ in $(seq 1 30); do
    id=$(container_id)
    [[ -n "$id" ]] || die "Gateway container is absent from project ${compose_project}"
    health=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$id")
    route=$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$id" | sed -n 's/^DIPOLE_GATEWAY_AGENT_PROMOTION_ENABLED=//p' | tail -n 1)
    [[ "$health" == "healthy" && "$route" == "$expected" ]] && break
    sleep 2
  done
  [[ "$health" == "healthy" ]] || die "Gateway is not healthy: ${health}"
  [[ "$route" == "$expected" ]] || die "Gateway promotion route is ${route:-unset}, expected ${expected}"
  printf 'Gateway verified: health=%s promotion_route=%s\n' "$health" "$route"
}

print_plan
if (( ! apply )); then
  printf 'Dry run only. Re-run with --apply after reviewing the maintenance window.\n'
  exit 0
fi

command -v docker >/dev/null 2>&1 || die "docker is required for --apply"
case "$action" in
  status)
    id=$(container_id)
    [[ -n "$id" ]] || die "Gateway container is absent from project ${compose_project}"
    expected=$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$id" | sed -n 's/^DIPOLE_GATEWAY_AGENT_PROMOTION_ENABLED=//p' | tail -n 1)
    [[ "$expected" == "true" || "$expected" == "false" ]] || die "Gateway promotion route is unset"
    assert_gateway_state "$expected"
    ;;
  open)
    DIPOLE_INTERNAL_CERT_DIR="$certificate_dir" \
      DIPOLE_GATEWAY_AGENT_PROMOTION_TENANT_ID="$tenant_id" \
      "${overlay_compose[@]}" config --quiet
    DIPOLE_INTERNAL_CERT_DIR="$certificate_dir" \
      DIPOLE_GATEWAY_AGENT_PROMOTION_TENANT_ID="$tenant_id" \
      "${overlay_compose[@]}" up -d --no-deps --force-recreate "$gateway_service"
    assert_gateway_state true
    ;;
  close)
    DIPOLE_INTERNAL_CERT_DIR="$certificate_dir" "${compose[@]}" config --quiet
    DIPOLE_INTERNAL_CERT_DIR="$certificate_dir" "${compose[@]}" up -d --no-deps --force-recreate "$gateway_service"
    assert_gateway_state false
    ;;
esac
