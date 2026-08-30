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
REMOTE_NODE_ROOT="${DIPOLE_REMOTE_NODE_ROOT:-/home/admin1/.local/node-22.12.0}"
REMOTE_K6_IMAGE="${DIPOLE_REMOTE_K6_IMAGE:-grafana/k6:0.57.0}"
REMOTE_BUILD_CANDIDATE="${DIPOLE_REMOTE_BUILD_CANDIDATE:-0}"

usage() {
  cat <<'EOF'
Usage: scripts/remote-dev.sh <sync|preflight|test|node-test|build|smoke-lite|multipart-smoke|multipart-restart-smoke|bench|down>

Environment: DIPOLE_REMOTE_HOST, DIPOLE_REMOTE_ROOT, DIPOLE_REMOTE_BRANCH,
  DIPOLE_REMOTE_PROJECT, DIPOLE_REMOTE_COMPOSE_FILE, DIPOLE_REMOTE_GO_ROOT,
  DIPOLE_REMOTE_GOPROXY, DIPOLE_REMOTE_NODE_ROOT, DIPOLE_REMOTE_BUILD_CANDIDATE.
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
  remote "${REMOTE_K6_IMAGE}" "${action}" "${REMOTE_NODE_ROOT}" "${REMOTE_GO_ROOT}" "${REMOTE_GOPROXY}" <<REMOTE_RUN
set -euo pipefail
root="\$1"; project="\$2"
k6_image="\${3:-}"
node_root="\${5:-}"
go_root="\${6:-}"
go_proxy="\${7:-}"
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
  node-test)
    if [[ -n "\$node_root" && -x "\$node_root/bin/node" ]]; then
      export PATH="\$node_root/bin:\$PATH"
    fi
    actual_node="\$(node --version 2>/dev/null || true)"
    if [[ -z "\$actual_node" ]]; then
      printf 'remote node-test refused: Node 22+ is unavailable; set DIPOLE_REMOTE_NODE_ROOT\n' >&2
      exit 4
    fi
    required_node="v22.0.0"
    if [[ "\$(printf '%s\n' "\$required_node" "\$actual_node" | sort -V | tail -n 1)" != "\$actual_node" ]]; then
      printf 'remote node-test refused: requires Node %s+, found %s\n' "\$required_node" "\$actual_node" >&2
      exit 4
    fi
    webapp_dir="internal/services/core/server/webapp"
    if [[ -n "\$(git status --porcelain -- "\$webapp_dir")" ]]; then
      printf 'remote node-test refused: generated webapp output is dirty; clean %s first\n' "\$webapp_dir" >&2
      exit 4
    fi
    cleanup_webapp() {
      if [[ -n "\$(git diff -- "\$webapp_dir")" ]]; then
        git diff -- "\$webapp_dir" | git apply --reverse || true
      fi
      untracked="\$(git ls-files --others --exclude-standard -- "\$webapp_dir")"
      if [[ -n "\$untracked" ]]; then
        git clean -f -- \$untracked
      fi
    }
    trap cleanup_webapp EXIT
    for app in services/agent-runtime frontend; do
      if [[ ! -d "\$app/node_modules" ]]; then
        npm --prefix "\$app" ci --ignore-scripts --no-audit --no-fund
      fi
      npm --prefix "\$app" install --include=optional --ignore-scripts --package-lock=false --no-audit --no-fund
      npm --prefix "\$app" test -- --run
      npm --prefix "\$app" run typecheck
      npm --prefix "\$app" run build
    done
    ;;
  build)
    scripts/docker-build.sh backend
    scripts/docker-build-microservice-images.sh
    if [[ "${REMOTE_BUILD_CANDIDATE}" == "1" ]]; then
      candidate_tag="dipole-server:c1-\$(git rev-parse --short HEAD)"
      candidate_revision="\$(git rev-parse HEAD)"
      candidate_created="\$(date -u +%Y-%m-%dT%H:%M:%SZ)"
      docker build \
        --build-arg DIPOLE_VCS_REVISION="\${candidate_revision}" \
        --build-arg DIPOLE_BUILD_CREATED="\${candidate_created}" \
        --build-arg DIPOLE_VCS_DIRTY=false \
        -t "\${candidate_tag}" .
      printf 'candidate image built: %s revision=%s\n' "\${candidate_tag}" "\${candidate_revision}"
    fi
    ;;
  smoke-lite) scripts/smoke-microservices-lite.sh ;;
  multipart-smoke)
    GOTOOLCHAIN=local scripts/smoke-minio-multipart.sh
    ;;
  multipart-restart-smoke)
    GOTOOLCHAIN=local scripts/smoke-minio-multipart-restart.sh
    ;;
  bench)
    bench_env=()
    if [[ "\$project" == dipole-c1* ]]; then
      bench_env=(
        BASE_URL=http://127.0.0.1:18081
        NODE1_WS=ws://127.0.0.1:18081
        NODE2_WS=ws://127.0.0.1:18082
        NODE1_HEALTH_URL=http://127.0.0.1:18081/health
        NODE2_HEALTH_URL=http://127.0.0.1:18082/health
        CONVERSATION_METRICS_SERVICES="dipole-node1 dipole-node2 dipole-node3"
        PROCESS_METRICS_SERVICES="dipole-node1 dipole-node2 dipole-node3"
      )
    fi
    if command -v k6 >/dev/null 2>&1; then
      env "\${bench_env[@]}" scripts/bench/run_bench.sh
    else
      [[ -n "\$k6_image" ]] || { echo "remote bench refused: k6 is unavailable and DIPOLE_REMOTE_K6_IMAGE is empty" >&2; exit 4; }
      docker image inspect "\$k6_image" >/dev/null 2>&1 || docker pull "\$k6_image"
      k6_wrapper="\$(mktemp)"
      cleanup_k6_wrapper() { rm -f "\$k6_wrapper"; }
      trap cleanup_k6_wrapper EXIT
      cat >"\$k6_wrapper" <<'K6_WRAPPER'
#!/usr/bin/env bash
set -euo pipefail
exec docker run --rm --network host --user "\$(id -u):\$(id -g)" -v "\$PWD:/workspace" -w /workspace "\${DIPOLE_K6_IMAGE}" "\$@"
K6_WRAPPER
      chmod 700 "\$k6_wrapper"
      env "\${bench_env[@]}" DIPOLE_K6_IMAGE="\$k6_image" K6_BIN="\$k6_wrapper" scripts/bench/run_bench.sh
    fi
    ;;
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
  node-test) sync_revision; run_remote node-test ;;
  build) sync_revision; guard_start; run_remote build ;;
  smoke-lite) sync_revision; guard_start; run_remote smoke-lite ;;
  multipart-smoke) sync_revision; run_remote multipart-smoke ;;
  multipart-restart-smoke) sync_revision; run_remote multipart-restart-smoke ;;
  bench) sync_revision; guard_start; run_remote bench ;;
  down) run_remote down ;;
  *) usage; exit 2 ;;
esac
