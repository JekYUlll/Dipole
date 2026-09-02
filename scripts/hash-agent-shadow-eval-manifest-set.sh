#!/usr/bin/env bash
set -euo pipefail

# Prints a content-only digest for a reviewed Shadow Eval manifest set. The
# digest deliberately excludes file names and manifest content from logs.
if [[ "$#" != "1" || -z "${1}" ]]; then
  echo "usage: hash-agent-shadow-eval-manifest-set.sh <manifest-directory>" >&2
  exit 2
fi

manifest_dir="$1"
[[ -d "${manifest_dir}" ]] || { echo "reviewed manifest directory is missing" >&2; exit 2; }

mapfile -d '' manifests < <(find "${manifest_dir}" -maxdepth 1 -type f -name '*.json' -print0 | sort -z)
(( ${#manifests[@]} > 0 )) || { echo "reviewed manifest directory has no JSON files" >&2; exit 2; }

digest_input="$(mktemp)"
trap 'rm -f "${digest_input}"' EXIT
for manifest in "${manifests[@]}"; do
  sha256sum "${manifest}" | awk '{print $1}' >>"${digest_input}"
done

sha256sum "${digest_input}" | awk '{print $1}'
