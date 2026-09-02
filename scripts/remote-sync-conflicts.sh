#!/usr/bin/env bash
set -euo pipefail

# Protect a reused remote candidate checkout before switching revisions. Test
# outputs can collide with a newly tracked visual baseline, but real remote
# edits must never be removed by an automated sync.
dipole_prepare_remote_checkout() {
  local target_commit="$1"
  local path expected_sha actual_sha
  local -a target_conflicts=()

  if ! git diff --quiet || ! git diff --cached --quiet; then
    printf 'remote sync refused: candidate checkout has tracked modifications\n' >&2
    return 2
  fi

  while IFS= read -r -d '' path; do
    if git cat-file -e "${target_commit}:${path}" 2>/dev/null; then
      target_conflicts+=("$path")
    fi
  done < <(git ls-files --others --exclude-standard -z)

  for path in "${target_conflicts[@]}"; do
    expected_sha="$(git show "${target_commit}:${path}" | sha256sum | awk '{print $1}')"
    actual_sha="$(sha256sum -- "$path" | awk '{print $1}')"
    if [[ "$actual_sha" != "$expected_sha" ]]; then
      printf 'remote sync refused: divergent untracked target path: %s\n' "$path" >&2
      return 3
    fi

    rm -- "$path"
    printf 'remote sync removed identical generated conflict: %s\n' "$path"
  done
}
