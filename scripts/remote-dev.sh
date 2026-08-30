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
BENCH_SCENARIO_FILTER="${DIPOLE_BENCH_SCENARIO_FILTER:-}"
BENCH_GROUP_MAX_DURATION="${DIPOLE_BENCH_GROUP_MAX_DURATION:-}"
BENCH_USER_COUNT="${DIPOLE_BENCH_USER_COUNT:-}"
BENCH_GROUP_SIZE="${DIPOLE_BENCH_GROUP_SIZE:-}"
BENCH_RUN_ID="${DIPOLE_BENCH_RUN_ID:-}"
BENCH_HOT_GROUP_WARMUP_MESSAGES="${DIPOLE_BENCH_HOT_GROUP_WARMUP_MESSAGES:-}"
BENCH_HOT_GROUP_ACTIVATION_WAIT_MS="${DIPOLE_BENCH_HOT_GROUP_ACTIVATION_WAIT_MS:-}"
BENCH_SCRIPT="${DIPOLE_BENCH_SCRIPT:-}"
BENCH_PHONE_PREFIX="${DIPOLE_BENCH_PHONE_PREFIX:-}"
BENCH_HOT_GROUP_MEMBER_COUNT_THRESHOLD="${DIPOLE_BENCH_HOT_GROUP_MEMBER_COUNT_THRESHOLD:-}"
BENCH_HOT_GROUP_MESSAGE_THRESHOLD="${DIPOLE_BENCH_HOT_GROUP_MESSAGE_THRESHOLD:-}"
REMOTE_EMPTY_ARG="__DIPOLE_EMPTY_ARG__"

usage() {
  cat <<'EOF'
Usage: scripts/remote-dev.sh <sync|preflight|test|node-test|build|smoke-lite|sync-ownership|web-sync-bundle|multipart-smoke|multipart-restart-smoke|bench|recovery|down>

Environment: DIPOLE_REMOTE_HOST, DIPOLE_REMOTE_ROOT, DIPOLE_REMOTE_BRANCH,
  DIPOLE_REMOTE_PROJECT, DIPOLE_REMOTE_COMPOSE_FILE, DIPOLE_REMOTE_GO_ROOT,
  DIPOLE_REMOTE_GOPROXY, DIPOLE_REMOTE_NODE_ROOT, DIPOLE_REMOTE_BUILD_CANDIDATE.
  Benchmark overrides: DIPOLE_BENCH_SCENARIO_FILTER, DIPOLE_BENCH_GROUP_MAX_DURATION,
  DIPOLE_BENCH_USER_COUNT, DIPOLE_BENCH_GROUP_SIZE, DIPOLE_BENCH_RUN_ID.
  DIPOLE_BENCH_HOT_GROUP_WARMUP_MESSAGES, DIPOLE_BENCH_HOT_GROUP_ACTIVATION_WAIT_MS.
  DIPOLE_BENCH_SCRIPT may select a repository-relative benchmark script.
  DIPOLE_BENCH_PHONE_PREFIX may select an isolated three-digit test namespace.
  DIPOLE_BENCH_HOT_GROUP_MEMBER_COUNT_THRESHOLD, DIPOLE_BENCH_HOT_GROUP_MESSAGE_THRESHOLD.
  Active login sessions remain blocked unless DIPOLE_REMOTE_ALLOW_ACTIVE=1 is set.
  Existing GPU tasks are recorded for resource planning and do not block CPU-only work.
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
  remote "guard" "${DIPOLE_REMOTE_ALLOW_ACTIVE:-0}" <<'REMOTE_GUARD'
set -euo pipefail
approved="${4:-0}"
users="$(who | wc -l | tr -d ' ')"
gpu="$(nvidia-smi --query-compute-apps=pid --format=csv,noheader 2>/dev/null | sed '/^[[:space:]]*$/d' | wc -l | tr -d ' ')"
if [[ "$users" != "0" && "$approved" != "1" ]]; then
  printf 'remote start refused: active_users=%s; set DIPOLE_REMOTE_ALLOW_ACTIVE=1 only with approval\n' "$users" >&2
  exit 3
fi
if [[ "$gpu" != "0" ]]; then
  printf 'remote start proceeding with existing GPU tasks: active_users=%s gpu_processes=%s\n' "$users" "$gpu" >&2
else
  printf 'remote start resource snapshot: active_users=%s gpu_processes=%s\n' "$users" "$gpu" >&2
fi
REMOTE_GUARD
}

run_remote() {
  local action="$1"
  local remote_k6_image="${REMOTE_K6_IMAGE:-$REMOTE_EMPTY_ARG}"
  local remote_node_root="${REMOTE_NODE_ROOT:-$REMOTE_EMPTY_ARG}"
  local remote_go_root="${REMOTE_GO_ROOT:-$REMOTE_EMPTY_ARG}"
  local remote_go_proxy="${REMOTE_GOPROXY:-$REMOTE_EMPTY_ARG}"
  local bench_scenario_filter="${BENCH_SCENARIO_FILTER:-$REMOTE_EMPTY_ARG}"
  local bench_group_max_duration="${BENCH_GROUP_MAX_DURATION:-$REMOTE_EMPTY_ARG}"
  local bench_user_count="${BENCH_USER_COUNT:-$REMOTE_EMPTY_ARG}"
  local bench_group_size="${BENCH_GROUP_SIZE:-$REMOTE_EMPTY_ARG}"
  local bench_run_id="${BENCH_RUN_ID:-$REMOTE_EMPTY_ARG}"
  local bench_hot_group_warmup_messages="${BENCH_HOT_GROUP_WARMUP_MESSAGES:-$REMOTE_EMPTY_ARG}"
  local bench_hot_group_activation_wait_ms="${BENCH_HOT_GROUP_ACTIVATION_WAIT_MS:-$REMOTE_EMPTY_ARG}"
  local bench_script="${BENCH_SCRIPT:-$REMOTE_EMPTY_ARG}"
  local bench_phone_prefix="${BENCH_PHONE_PREFIX:-$REMOTE_EMPTY_ARG}"
  local bench_hot_group_member_count_threshold="${BENCH_HOT_GROUP_MEMBER_COUNT_THRESHOLD:-$REMOTE_EMPTY_ARG}"
  local bench_hot_group_message_threshold="${BENCH_HOT_GROUP_MESSAGE_THRESHOLD:-$REMOTE_EMPTY_ARG}"
  remote "${remote_k6_image}" "${action}" "${remote_node_root}" "${remote_go_root}" "${remote_go_proxy}" \
    "${bench_scenario_filter}" "${bench_group_max_duration}" "${bench_user_count}" "${bench_group_size}" "${bench_run_id}" \
    "${bench_hot_group_warmup_messages}" "${bench_hot_group_activation_wait_ms}" "${bench_script}" "${bench_phone_prefix}" \
    "${bench_hot_group_member_count_threshold}" "${bench_hot_group_message_threshold}" <<REMOTE_RUN
set -euo pipefail
root="\$1"; project="\$2"
k6_image="\${3:-}"
node_root="\${5:-}"
go_root="\${6:-}"
go_proxy="\${7:-}"
bench_scenario_filter="\${8:-}"
bench_group_max_duration="\${9:-}"
bench_user_count="\${10:-}"
bench_group_size="\${11:-}"
bench_run_id="\${12:-}"
bench_hot_group_warmup_messages="\${13:-}"
bench_hot_group_activation_wait_ms="\${14:-}"
bench_script="\${15:-}"
bench_phone_prefix="\${16:-}"
bench_hot_group_member_count_threshold="\${17:-}"
bench_hot_group_message_threshold="\${18:-}"
for bench_arg in k6_image node_root go_root go_proxy bench_scenario_filter bench_group_max_duration bench_user_count bench_group_size bench_run_id bench_hot_group_warmup_messages bench_hot_group_activation_wait_ms bench_script bench_phone_prefix bench_hot_group_member_count_threshold bench_hot_group_message_threshold; do
  [[ "\${!bench_arg}" == "${REMOTE_EMPTY_ARG}" ]] && printf -v "\$bench_arg" '%s' ''
done
if [[ -z "\$go_root" ]]; then
  selected_go=""
  selected_version=""
  while IFS= read -r candidate_go; do
    candidate_version="\$(GOTOOLCHAIN=local "\$candidate_go" version 2>/dev/null | awk '{print \$3}')"
    [[ "\$candidate_version" == go[0-9]* ]] || continue
    if [[ -z "\$selected_version" || "\$(printf '%s\n' "\$selected_version" "\$candidate_version" | sort -V | tail -n 1)" == "\$candidate_version" ]]; then
      selected_go="\$candidate_go"
      selected_version="\$candidate_version"
    fi
  done < <(find /home/admin1/.local -maxdepth 4 -type f -path '*/bin/go' -perm -111 -print 2>/dev/null | sort -V)
  if [[ -n "\$selected_go" ]]; then
    go_root="\${selected_go%/bin/go}"
    printf 'remote Go toolchain auto-selected: root=%s version=%s\n' "\$go_root" "\$selected_version" >&2
  fi
fi
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
  sync-ownership)
    GOTOOLCHAIN=local scripts/smoke-sync-write-ownership.sh
    ;;
  web-sync-bundle)
    bundle="/tmp/\${project}-web-sync-shadow-\$(git rev-parse --short HEAD).tar"
    scripts/package-web-sync-bundle.sh \
      --candidate-version "web-sync-shadow-\$(git rev-parse --short HEAD)" \
      --mode shadow \
      --output "\$bundle"
    ;;
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
    [[ -n "\$bench_scenario_filter" ]] && bench_env+=(SCENARIO_FILTER="\$bench_scenario_filter")
    [[ -n "\$bench_group_max_duration" ]] && bench_env+=(GROUP_MAX_DURATION="\$bench_group_max_duration")
    [[ -n "\$bench_user_count" ]] && bench_env+=(USER_COUNT="\$bench_user_count")
    [[ -n "\$bench_group_size" ]] && bench_env+=(GROUP_SIZE="\$bench_group_size")
    [[ -n "\$bench_run_id" ]] && bench_env+=(RUN_ID="\$bench_run_id")
    [[ -n "\$bench_hot_group_warmup_messages" ]] && bench_env+=(HOT_GROUP_WARMUP_MESSAGES="\$bench_hot_group_warmup_messages")
    [[ -n "\$bench_hot_group_activation_wait_ms" ]] && bench_env+=(HOT_GROUP_ACTIVATION_WAIT_MS="\$bench_hot_group_activation_wait_ms")
    [[ -n "\$bench_script" ]] && bench_env+=(BENCH_SCRIPT="\$bench_script")
    [[ -n "\$bench_phone_prefix" ]] && bench_env+=(PHONE_PREFIX="\$bench_phone_prefix")
    [[ -n "\$bench_hot_group_member_count_threshold" ]] && bench_env+=(HOT_GROUP_MEMBER_COUNT_THRESHOLD="\$bench_hot_group_member_count_threshold")
    [[ -n "\$bench_hot_group_message_threshold" ]] && bench_env+=(HOT_GROUP_MESSAGE_THRESHOLD="\$bench_hot_group_message_threshold")
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
exec docker run --rm --network host --user "\$(id -u):\$(id -g)" -v "\$PWD:/workspace" -v /tmp:/tmp -w /workspace "\${DIPOLE_K6_IMAGE}" "\$@"
K6_WRAPPER
      chmod 700 "\$k6_wrapper"
      env "\${bench_env[@]}" DIPOLE_K6_IMAGE="\$k6_image" K6_BIN="\$k6_wrapper" scripts/bench/run_bench.sh
    fi
    ;;
  recovery)
    recovery_env=(
      COMPOSE_PROJECT_NAME="\$project"
      COMPOSE_FILE="deploy/compose/docker-compose.dist.yml"
      TARGET_SERVICE=dipole-node2
      RESULTS_DIR="/tmp/\${project}-recovery"
      RUN_ID="\${bench_run_id:-recovery-\$(git rev-parse --short HEAD)}"
      BASE_URL=http://127.0.0.1:18081
      NODE1_WS=ws://127.0.0.1:18081
      NODE2_WS=ws://127.0.0.1:18082
      NODE1_HEALTH_URL=http://127.0.0.1:18081/health
      NODE2_HEALTH_URL=http://127.0.0.1:18082/health
      NODE3_HEALTH_URL=http://127.0.0.1:18083/health
      USER_COUNT="\${bench_user_count:-20}"
      PHONE_PREFIX="\${bench_phone_prefix:-136}"
    )
    if command -v k6 >/dev/null 2>&1; then
      env "\${recovery_env[@]}" K6_BIN=k6 scripts/bench/recovery_drill.sh
    else
      [[ -n "\$k6_image" ]] || { echo "remote recovery refused: k6 is unavailable and DIPOLE_REMOTE_K6_IMAGE is empty" >&2; exit 4; }
      docker image inspect "\$k6_image" >/dev/null 2>&1 || docker pull "\$k6_image"
      k6_wrapper="\$(mktemp)"
      cleanup_k6_wrapper() { rm -f "\$k6_wrapper"; }
      trap cleanup_k6_wrapper EXIT
      cat >"\$k6_wrapper" <<'K6_WRAPPER'
#!/usr/bin/env bash
set -euo pipefail
exec docker run --rm --network host --user "\$(id -u):\$(id -g)" -v "\$PWD:/workspace" -v /tmp:/tmp -w /workspace "\${DIPOLE_K6_IMAGE}" "\$@"
K6_WRAPPER
      chmod 700 "\$k6_wrapper"
      env "\${recovery_env[@]}" DIPOLE_K6_IMAGE="\$k6_image" K6_BIN="\$k6_wrapper" scripts/bench/recovery_drill.sh
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
  sync-ownership) sync_revision; run_remote sync-ownership ;;
  web-sync-bundle) sync_revision; run_remote web-sync-bundle ;;
  multipart-smoke) sync_revision; run_remote multipart-smoke ;;
  multipart-restart-smoke) sync_revision; run_remote multipart-restart-smoke ;;
  bench) sync_revision; guard_start; run_remote bench ;;
  recovery) sync_revision; guard_start; run_remote recovery ;;
  down) run_remote down ;;
  *) usage; exit 2 ;;
esac
