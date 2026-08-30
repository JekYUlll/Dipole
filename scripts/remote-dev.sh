#!/usr/bin/env bash
set -euo pipefail

# Remote development entrypoint. It never copies an uncommitted worktree.
REMOTE_HOST="${DIPOLE_REMOTE_HOST:-LAB113-OPS}"
REMOTE_ROOT="${DIPOLE_REMOTE_ROOT:-/home/admin1/workspaces/Dipole}"
REMOTE_BRANCH="${DIPOLE_REMOTE_BRANCH:-dipole-dev/${USER:-developer}}"
REMOTE_PROJECT="${DIPOLE_REMOTE_PROJECT:-dipole-dev-${USER:-developer}}"
REMOTE_COMPOSE_FILE="${DIPOLE_REMOTE_COMPOSE_FILE:-deploy/compose/docker-compose.microservices.yml}"
REMOTE_GO_ROOT="${DIPOLE_REMOTE_GO_ROOT:-}"
REMOTE_GOPROXY="${DIPOLE_REMOTE_GOPROXY:-}"

usage() {
  cat <<'EOF'
Usage: scripts/remote-dev.sh <sync|preflight|test|build|smoke-lite|bench|down>

Environment: DIPOLE_REMOTE_HOST, DIPOLE_REMOTE_ROOT, DIPOLE_REMOTE_BRANCH,
  DIPOLE_REMOTE_PROJECT, DIPOLE_REMOTE_COMPOSE_FILE, DIPOLE_REMOTE_GO_ROOT,
  DIPOLE_REMOTE_GOPROXY.
  Set DIPOLE_REMOTE_ALLOW_ACTIVE=1 only during an explicitly approved window.
EOF
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || { echo "required command not found: $1" >&2; exit 2; }
}

remote() {
  ssh -o BatchMode=yes -o ConnectTimeout="${DIPOLE_REMOTE_CONNECT_TIMEOUT:-8}" \
    "${REMOTE_HOST}" bash -s -- "${REMOTE_ROOT}" "${REMOTE_PROJECT}" "$@"
}

sync_revision() {
  [[ -z "$(git status --porcelain)" ]] || { echo "commit or stash local changes before remote sync" >&2; exit 2; }
  local commit remote_url
  commit="$(git rev-parse HEAD)"
  remote_url="$(git remote get-url origin)"
  git push origin "${commit}:refs/heads/${REMOTE_BRANCH}"
  remote "${REMOTE_BRANCH}" "${commit}" "${remote_url}" <<'REMOTE_SYNC'
set -euo pipefail
root="$1"; project="$2"; branch="$3"; commit="$4"; remote_url="$5"
mkdir -p "$(dirname "$root")"
if [[ ! -d "$root/.git" ]]; then
  git clone "$remote_url" "$root"
fi
cd "$root"
git fetch origin "refs/heads/${branch}:refs/remotes/origin/${branch}" || true
git checkout --detach "$commit"
printf 'remote source ready: commit=%s root=%s\n' "$commit" "$root"
REMOTE_SYNC
}

guard_start() {
  [[ "${DIPOLE_REMOTE_ALLOW_ACTIVE:-0}" == "1" ]] || {
    remote "guard" <<'REMOTE_GUARD'
set -euo pipefail
users="$(who | wc -l | tr -d ' ')"
gpu="$(nvidia-smi --query-compute-apps=pid --format=csv,noheader 2>/dev/null | sed '/^[[:space:]]*$/d' | wc -l | tr -d ' ')"
if [[ "$users" != "0" || "$gpu" != "0" ]]; then
  printf 'remote start refused: users=%s gpu_processes=%s; set DIPOLE_REMOTE_ALLOW_ACTIVE=1 only with approval\n' "$users" "$gpu" >&2
  exit 3
fi
REMOTE_GUARD
  }
}

run_remote() {
  local action="$1"
  remote "${action}" "${REMOTE_GO_ROOT}" "${REMOTE_GOPROXY}" <<REMOTE_RUN
set -euo pipefail
root="\$1"; project="\$2"
go_root="\${4:-}"
go_proxy="\${5:-}"
if [[ -n "\$go_root" && -x "\$go_root/bin/go" ]]; then
  export PATH="\$go_root/bin:\$PATH"
fi
if [[ -n "\$go_proxy" ]]; then
  export GOPROXY="\$go_proxy"
fi
cd "\$root"
export COMPOSE_PROJECT_NAME="\$project"
export DIPOLE_HOST_PROFILE=remote-gpu
case "${action}" in
  preflight) scripts/check-dev-host.sh remote-gpu ;;
  test)
    required_go="go1.26"
    actual_go="\$(GOTOOLCHAIN=local go env GOVERSION 2>/dev/null || true)"
    if [[ -z "\$actual_go" ]]; then
      printf 'remote test refused: local Go toolchain is unavailable; install %s or newer\n' "\$required_go" >&2
      exit 4
    fi
    if [[ "\$(printf '%s\n' "\$required_go" "\$actual_go" | sort -V | tail -n 1)" != "\$actual_go" ]]; then
      printf 'remote test refused: requires Go %s+, found %s; implicit toolchain download is disabled\n' "\$required_go" "\$actual_go" >&2
      exit 4
    fi
    GOTOOLCHAIN=local scripts/check-go.sh && scripts/check-compose.sh && scripts/check-service-layout.sh && scripts/check-architecture-docs.sh
    ;;
  build) scripts/docker-build-microservice-images.sh ;;
  smoke-lite) scripts/smoke-microservices-lite.sh ;;
  bench) scripts/bench/run_bench.sh ;;
  down) docker compose -p "\$project" -f "${REMOTE_COMPOSE_FILE}" down --remove-orphans ;;
  *) echo "unsupported remote action: ${action}" >&2; exit 2 ;;
esac
REMOTE_RUN
}

require_command git
require_command ssh
case "${1:-}" in
  sync) sync_revision ;;
  preflight) run_remote preflight ;;
  test) sync_revision; run_remote test ;;
  build) sync_revision; guard_start; run_remote build ;;
  smoke-lite) sync_revision; guard_start; run_remote smoke-lite ;;
  bench) sync_revision; guard_start; run_remote bench ;;
  down) run_remote down ;;
  *) usage; exit 2 ;;
esac
