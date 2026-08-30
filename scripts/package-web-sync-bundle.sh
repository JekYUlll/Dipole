#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
source_dir="$root_dir/internal/services/core/server/webapp"
candidate_version=""
mode="shadow"
output=""

usage() {
  printf 'Usage: %s --candidate-version VERSION --output FILE [--source-dir DIR] [--mode shadow|primary|off]\n' "$0" >&2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --candidate-version) candidate_version="${2:-}"; shift 2 ;;
    --output) output="${2:-}"; shift 2 ;;
    --source-dir) source_dir="${2:-}"; shift 2 ;;
    --mode) mode="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) printf 'unknown argument: %s\n' "$1" >&2; usage; exit 2 ;;
  esac
done

[[ "$candidate_version" =~ ^[A-Za-z0-9._-]+$ ]] || { echo 'candidate version must be a safe token' >&2; exit 2; }
[[ "$mode" == shadow || "$mode" == primary || "$mode" == off ]] || { echo 'mode must be shadow, primary, or off' >&2; exit 2; }
[[ -n "$output" ]] || { echo '--output is required' >&2; exit 2; }
[[ -d "$source_dir" ]] || { printf 'bundle source directory does not exist: %s\n' "$source_dir" >&2; exit 2; }
[[ -n "$(find "$source_dir" -type f -print -quit)" ]] || { echo 'bundle source directory is empty' >&2; exit 2; }
[[ ! -e "$output" ]] || { printf 'refusing to overwrite bundle: %s\n' "$output" >&2; exit 3; }
command -v realpath >/dev/null || { echo 'realpath is required' >&2; exit 2; }
source_real=$(realpath "$source_dir")
output_real=$(realpath -m "$output")
[[ "$output_real" != "$source_real"/* ]] || { echo 'bundle output must be outside the source directory' >&2; exit 3; }

git -C "$root_dir" diff --quiet || { echo 'bundle requires a clean worktree' >&2; exit 3; }
git -C "$root_dir" diff --cached --quiet || { echo 'bundle requires an unstaged worktree' >&2; exit 3; }
command -v tar >/dev/null || { echo 'tar is required' >&2; exit 2; }
command -v sha256sum >/dev/null || { echo 'sha256sum is required' >&2; exit 2; }

revision=$(git -C "$root_dir" rev-parse HEAD)
output_dir=$(dirname "$output")
mkdir -p "$output_dir"
staging_dir=$(mktemp -d "${TMPDIR:-/tmp}/dipole-web-sync-bundle.XXXXXX")
temporary_output=$(mktemp "${output}.tmp.XXXXXX")
cleanup() {
  rm -rf "$staging_dir"
  rm -f "$temporary_output"
}
trap cleanup EXIT INT TERM

cp -a "$source_dir"/. "$staging_dir"/
printf '{"schema_version":"dipole.web-sync.bundle.v1","candidate_version":"%s","git_commit":"%s","mode":"%s"}\n' \
  "$candidate_version" "$revision" "$mode" >"$staging_dir/web-sync-bundle.json"

tar --sort=name --mtime='UTC 1970-01-01' --owner=0 --group=0 --numeric-owner \
  -C "$staging_dir" -cf "$temporary_output" .
chmod 0600 "$temporary_output"
mv -f "$temporary_output" "$output"
printf 'Web Sync bundle created: path=%s revision=%s mode=%s sha256=%s\n' \
  "$output" "$revision" "$mode" "$(sha256sum "$output" | awk '{print $1}')"
