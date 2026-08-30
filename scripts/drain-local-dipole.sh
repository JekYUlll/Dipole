#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/drain-local-dipole.sh [--dry-run|--apply]

Stops running Docker containers whose names start with "dipole".
It never removes containers, volumes, images, or unrelated projects.
EOF
}

mode="${1:---dry-run}"
case "$mode" in
  --dry-run|--apply) ;;
  -h|--help) usage; exit 0 ;;
  *) usage >&2; exit 2 ;;
esac

targets="$(docker ps --format '{{.ID}}\t{{.Names}}' | awk '$2 ~ /^dipole/ {print}')"
if [[ -z "$targets" ]]; then
  echo "local Dipole containers: none"
  exit 0
fi

printf 'local Dipole containers:\n%s\n' "$targets"
if [[ "$mode" == "--dry-run" ]]; then
  echo "dry-run: no containers stopped"
  exit 0
fi

while IFS=$'\t' read -r id name; do
  [[ -z "$id" || -z "$name" ]] && continue
  printf 'stopping %s\n' "$name"
  docker stop "$id" >/dev/null
done <<< "$targets"

remaining="$(docker ps --format '{{.Names}}' | awk '/^dipole/ {print}')"
if [[ -n "$remaining" ]]; then
  printf 'local Dipole containers still running:\n%s\n' "$remaining" >&2
  exit 1
fi
echo "local Dipole containers stopped; volumes and images retained"
