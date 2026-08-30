#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

index_files=(README.md docs/README.md docs/agent/README.md contracts/README.md)
for index in "${index_files[@]}"; do
  if [[ ! -f "${index}" ]]; then
    echo "document index is missing: ${index}" >&2
    exit 1
  fi

  while IFS= read -r link; do
    link="${link%%\#*}"
    link="${link%% *}"
    [[ -z "${link}" || "${link}" == http://* || "${link}" == https://* || "${link}" == mailto:* ]] && continue
    if [[ ! -e "$(dirname "${index}")/${link}" ]]; then
      echo "document index link is broken: ${index} -> ${link}" >&2
      exit 1
    fi
  done < <(sed -nE 's/.*\]\(([^)]*)\).*/\1/p' "${index}")
done

echo "document index link gate passed"
