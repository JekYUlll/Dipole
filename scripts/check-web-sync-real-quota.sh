#!/usr/bin/env bash
set -euo pipefail

ROOT="$(mktemp -d /tmp/dipole-web-quota.XXXXXX)"
cleanup() {
  rmdir "$ROOT" 2>/dev/null || true
}
trap cleanup EXIT

export DIPOLE_QUOTA_ROOT="$ROOT"
unshare -Urm bash -c '
  set -euo pipefail
  mount -t tmpfs -o size=128m tmpfs "$DIPOLE_QUOTA_ROOT"
  cleanup_namespace() {
    umount "$DIPOLE_QUOTA_ROOT" 2>/dev/null || true
  }
  trap cleanup_namespace EXIT
  fallocate -l 24m "$DIPOLE_QUOTA_ROOT/reserve.bin"
  cd frontend
  DIPOLE_REAL_QUOTA_PROFILE_ROOT="$DIPOLE_QUOTA_ROOT" \
  DIPOLE_REAL_QUOTA_RESERVE_FILE="$DIPOLE_QUOTA_ROOT/reserve.bin" \
    npx playwright test e2e/indexeddb.spec.ts \
      --project=chromium \
      --grep "constrained profile filesystem"
'
