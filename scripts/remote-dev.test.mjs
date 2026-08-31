import assert from "node:assert/strict";
import childProcess from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";

const source = fs.readFileSync(new URL("./remote-dev.sh", import.meta.url), "utf8");
const conflictHelper = new URL("./remote-sync-conflicts.sh", import.meta.url);

function git(cwd, ...args) {
  return childProcess.execFileSync("git", args, { cwd, encoding: "utf8" }).trim();
}

function runConflictHelper(cwd, target) {
  return childProcess.spawnSync(
    "bash",
    ["-c", 'source <(git show "${1}:scripts/remote-sync-conflicts.sh"); dipole_prepare_remote_checkout "$1"', "--", target],
    { cwd, encoding: "utf8" },
  );
}

function createConflictFixture() {
  const cwd = fs.mkdtempSync(path.join(os.tmpdir(), "dipole-remote-sync-"));
  git(cwd, "init", "--quiet");
  git(cwd, "config", "user.name", "Dipole Test");
  git(cwd, "config", "user.email", "dipole-test@example.invalid");
  fs.writeFileSync(path.join(cwd, "README.md"), "initial\n");
  fs.mkdirSync(path.join(cwd, "scripts"), { recursive: true });
  fs.writeFileSync(path.join(cwd, "scripts", "remote-sync-conflicts.sh"), fs.readFileSync(conflictHelper));
  git(cwd, "add", "README.md", "scripts/remote-sync-conflicts.sh");
  git(cwd, "commit", "--quiet", "-m", "initial");
  const base = git(cwd, "rev-parse", "HEAD");

  fs.mkdirSync(path.join(cwd, "frontend", "e2e", "__screenshots__"), { recursive: true });
  fs.writeFileSync(path.join(cwd, "frontend", "e2e", "__screenshots__", "settings.png"), "approved-baseline\n");
  git(cwd, "add", "frontend/e2e/__screenshots__/settings.png");
  git(cwd, "commit", "--quiet", "-m", "add baseline");
  const target = git(cwd, "rev-parse", "HEAD");
  git(cwd, "checkout", "--quiet", "--detach", base);
  return { cwd, target };
}

test("remote sync only accepts a clean committed worktree", () => {
  assert.match(source, /sync_revision\(\)[\s\S]*?git status --porcelain/);
  assert.match(source, /commit or stash local changes before remote sync/);
});

test("per-user candidate refs use an exact lease while shared refs stay fast-forward only", () => {
  assert.match(source, /git ls-remote --heads origin "refs\/heads\/\$\{REMOTE_BRANCH\}"/);
  assert.match(source, /REMOTE_BRANCH\}" == "dipole-dev\/"\*/);
  assert.match(source, /--force-with-lease="refs\/heads\/\$\{REMOTE_BRANCH\}:\$\{remote_tip\}"/);
  assert.match(source, /else\n    git push origin "\$\{commit\}:refs\/heads\/\$\{REMOTE_BRANCH\}"/);
});

test("remote candidate tracking refs accept only the expected mutable update", () => {
  assert.match(source, /if \[\[ "\$branch" == dipole-dev\/\* \]\]; then[\s\S]*?fetch_ref="\+refs\/heads\/\$\{branch\}:refs\/remotes\/origin\/\$\{branch\}"/);
  assert.match(source, /else\n  fetch_ref="refs\/heads\/\$\{branch\}:refs\/remotes\/origin\/\$\{branch\}"/);
  assert.match(source, /if ! timeout "\$git_timeout" git fetch origin "\$fetch_ref"; then/);
  assert.doesNotMatch(source, /git fetch origin "refs\/heads\/\$\{branch\}:refs\/remotes\/origin\/\$\{branch\}" \|\| true/);
});

test("remote sync carries a commit-pinned bundle for outbound Git fallback", () => {
  assert.match(source, /git bundle create "\$\{bundle_path\}" "\$\{commit\}"/);
  assert.match(source, /scp -q -o BatchMode=yes -o ConnectTimeout=[\s\S]*?"\$\{bundle_path\}" "\$\{REMOTE_HOST\}:\$\{remote_bundle\}"/);
  assert.match(source, /git_timeout="\$\{DIPOLE_REMOTE_GIT_TIMEOUT:-20\}"/);
  assert.match(source, /if ! timeout "\$git_timeout" git clone "\$remote_url" "\$root"; then[\s\S]*?git clone "\$bundle" "\$root"/);
  assert.match(source, /if ! timeout "\$git_timeout" git fetch origin "\$fetch_ref"; then[\s\S]*?git fetch "\$bundle" "\$commit"/);
  assert.match(source, /cleanup_bundle\(\) \{ rm -f "\$bundle"; \}/);
  assert.match(source, /git rev-parse --verify "\$\{commit\}\^\{commit\}"/);
  assert.match(source, /require_command scp/);
  assert.match(source, /require_command timeout/);
});

test("remote checkout prepares only verified generated conflicts before switching revisions", () => {
  assert.ok(source.includes('source <(git show "${commit}:scripts/remote-sync-conflicts.sh")'));
  assert.match(source, /source <\(git show[\s\S]*?dipole_prepare_remote_checkout "\$commit"[\s\S]*?git checkout --detach "\$commit"/);
});

test("remote sync removes only an identical untracked target conflict", (t) => {
  const { cwd, target } = createConflictFixture();
  t.after(() => fs.rmSync(cwd, { recursive: true, force: true }));
  const snapshot = path.join(cwd, "frontend", "e2e", "__screenshots__", "settings.png");
  fs.mkdirSync(path.dirname(snapshot), { recursive: true });
  fs.writeFileSync(snapshot, "approved-baseline\n");

  const result = runConflictHelper(cwd, target);

  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /removed identical generated conflict/);
  assert.equal(fs.existsSync(snapshot), false);
});

test("remote sync preserves divergent untracked target conflicts and tracked edits", (t) => {
  const { cwd, target } = createConflictFixture();
  t.after(() => fs.rmSync(cwd, { recursive: true, force: true }));
  const snapshot = path.join(cwd, "frontend", "e2e", "__screenshots__", "settings.png");
  fs.mkdirSync(path.dirname(snapshot), { recursive: true });
  fs.writeFileSync(snapshot, "remote-local-difference\n");

  const divergent = runConflictHelper(cwd, target);

  assert.equal(divergent.status, 3);
  assert.match(divergent.stderr, /divergent untracked target path/);
  assert.equal(fs.readFileSync(snapshot, "utf8"), "remote-local-difference\n");

  fs.rmSync(snapshot);
  fs.writeFileSync(path.join(cwd, "README.md"), "remote tracked edit\n");
  const tracked = runConflictHelper(cwd, target);

  assert.equal(tracked.status, 2);
  assert.match(tracked.stderr, /tracked modifications/);
  assert.equal(fs.readFileSync(path.join(cwd, "README.md"), "utf8"), "remote tracked edit\n");
});

test("node verification preserves package locks and cleans generated webapp output", () => {
  assert.match(source, /node-test\)[\s\S]*?--package-lock=false/);
  assert.match(source, /webapp_dir="internal\/services\/core\/server\/webapp"/);
  assert.match(source, /generated webapp output is dirty/);
  assert.match(source, /trap cleanup_webapp EXIT/);
});

test("remote destructive actions remain behind the active-user guard", () => {
  assert.match(source, /build\) sync_revision; guard_start; run_remote build/);
  assert.match(source, /smoke-lite\) sync_revision; guard_start; run_remote smoke-lite/);
  assert.match(source, /bench\) sync_revision; guard_start; run_remote bench/);
  assert.match(source, /recovery\) sync_revision; guard_start; run_remote recovery/);
  assert.match(source, /web-sync-observability-smoke\) sync_revision; guard_start; run_remote web-sync-observability-smoke/);
});

test("web sync observability smoke preserves its remote compose project variable", () => {
  assert.match(source, /web-sync-observability-smoke\)[\s\S]*?COMPOSE_PROJECT_NAME="\\\$\{project\}"/);
  assert.match(source, /web-sync-observability-smoke\)[\s\S]*?DIPOLE_GATEWAY_PORT=18080/);
  assert.match(source, /web-sync-observability-smoke\)[\s\S]*?DIPOLE_PROMETHEUS_PORT=19090/);
  assert.match(source, /web-sync-observability-smoke\)[\s\S]*?DIPOLE_ALERTMANAGER_PORT=19093/);
});

test("recovery entry uses candidate ports and a temporary report directory", () => {
  assert.match(source, /recovery\)[\s\S]*?COMPOSE_FILE="deploy\/compose\/docker-compose\.dist\.yml"/);
  assert.ok(source.includes('RESULTS_DIR="/tmp/\\${project}-recovery"'));
  assert.match(source, /recovery\)[\s\S]*?TARGET_SERVICE=dipole-node2/);
  assert.match(source, /recovery\)[\s\S]*?K6_BIN="\\\$k6_wrapper" scripts\/bench\/recovery_drill\.sh/);
});

test("existing GPU tasks are observed without blocking development actions", () => {
  assert.match(source, /active_users=.*gpu_processes=.*\\n/);
  assert.match(source, /remote "guard" "\$\{DIPOLE_REMOTE_ALLOW_ACTIVE:-0\}"/);
  assert.match(source, /approved="\$\{4:-0\}"/);
  assert.match(source, /active_users.*DIPOLE_REMOTE_ALLOW_ACTIVE/);
  assert.doesNotMatch(source, /if \[\[ "\$users" != "0" \|\| "\$gpu" != "0"/);
  assert.match(source, /proceeding with existing GPU tasks/);
});

test("multipart smoke is isolated and does not require a GPU-free host", () => {
  assert.match(source, /multipart-smoke\) sync_revision; run_remote multipart-smoke/);
  assert.match(source, /multipart-smoke\)[\s\S]*?GOTOOLCHAIN=local scripts\/smoke-minio-multipart\.sh/);
});

test("sync ownership smoke is available through the remote CPU workflow", () => {
  assert.match(source, /sync-ownership\) sync_revision; run_remote sync-ownership/);
  assert.match(source, /sync-ownership\)[\s\S]*?GOTOOLCHAIN=local scripts\/smoke-sync-write-ownership\.sh/);
  assert.doesNotMatch(source, /sync-ownership\) sync_revision; guard_start/);
});

test("web sync bundle packaging is available as a shadow-only remote action", () => {
  assert.match(source, /web-sync-bundle\) sync_revision; run_remote web-sync-bundle/);
  assert.match(source, /web-sync-bundle\)[\s\S]*?--mode shadow/);
  assert.match(source, /--candidate-version "web-sync-shadow-/);
  assert.ok(source.includes('bundle="/tmp/\\${project}-web-sync-shadow-'));
  assert.doesNotMatch(source, /web-sync-bundle\) sync_revision; guard_start/);
});

test("direct multipart smoke scripts honor an explicit remote Go toolchain", () => {
  for (const name of ["smoke-minio-multipart.sh", "smoke-minio-multipart-restart.sh"]) {
    const smoke = fs.readFileSync(new URL(`./${name}`, import.meta.url), "utf8");
    assert.match(smoke, /DIPOLE_REMOTE_GO_ROOT/);
    assert.match(smoke, /does not contain an executable Go binary/);
    assert.match(smoke, /export PATH=.*DIPOLE_REMOTE_GO_ROOT/);
    assert.match(smoke, /export GOTOOLCHAIN=.*local/);
  }
});

test("remote image builds compile committed backend binaries first", () => {
  assert.match(source, /build\)[\s\S]*?scripts\/docker-build\.sh backend[\s\S]*?scripts\/docker-build-microservice-images\.sh/);
});

test("benchmark uses an explicit k6 binary and has a Docker fallback on remote hosts", () => {
  const bench = fs.readFileSync(new URL("./bench/run_bench.sh", import.meta.url), "utf8");
  assert.match(bench, /K6_BIN="\$\{K6_BIN:-k6\}"/);
  assert.match(bench, /require_command "\$\{K6_BIN\}"/);
  assert.match(bench, /"\$\{K6_BIN\}" run/);
  assert.match(source, /REMOTE_K6_IMAGE="\$\{DIPOLE_REMOTE_K6_IMAGE:-grafana\/k6:0\.57\.0\}"/);
  assert.match(source, /docker run --rm --network host --user/);
  assert.match(source, /-v \/tmp:\/tmp -w \/workspace/);
  assert.match(source, /DIPOLE_K6_IMAGE\}" "\\\$@"/);
  assert.doesNotMatch(source, /DIPOLE_K6_IMAGE\}" k6 "\\\$@"/);
  assert.match(source, /K6_BIN="\\\$k6_wrapper" scripts\/bench\/run_bench\.sh/);
  assert.match(source, /remote "\$\{remote_k6_image\}" "\$\{action\}"/);
  assert.match(source, /k6_image="\\\$\{3:-\}"/);
  assert.match(source, /BASE_URL=http:\/\/127\.0\.0\.1:18081/);
  assert.match(source, /NODE2_HEALTH_URL=http:\/\/127\.0\.0\.1:18082\/health/);
  assert.match(source, /if \[\[ "\\\$project" == dipole-c1\* \]\]/);
});

test("benchmark workload overrides are forwarded through an explicit allowlist", () => {
  assert.ok(source.includes('BENCH_SCENARIO_FILTER="${DIPOLE_BENCH_SCENARIO_FILTER:-}"'));
  assert.ok(source.includes('BENCH_GROUP_MAX_DURATION="${DIPOLE_BENCH_GROUP_MAX_DURATION:-}"'));
  assert.ok(source.includes('bench_scenario_filter="\\${8:-}"'));
  assert.ok(source.includes('bench_group_size="\\${11:-}"'));
  assert.ok(source.includes('bench_env+=(SCENARIO_FILTER="\\$bench_scenario_filter")'));
  assert.ok(source.includes('bench_env+=(GROUP_SIZE="\\$bench_group_size")'));
  assert.ok(source.includes('bench_env+=(RUN_ID="\\$bench_run_id")'));
});

test("benchmark positional forwarding preserves empty optional values", () => {
  assert.ok(source.includes('REMOTE_EMPTY_ARG="__DIPOLE_EMPTY_ARG__"'));
  assert.ok(source.includes('local remote_go_proxy="${REMOTE_GOPROXY:-$REMOTE_EMPTY_ARG}"'));
  assert.ok(source.includes('local bench_scenario_filter="${BENCH_SCENARIO_FILTER:-$REMOTE_EMPTY_ARG}"'));
  assert.ok(source.includes('local bench_hot_group_warmup_messages="${BENCH_HOT_GROUP_WARMUP_MESSAGES:-$REMOTE_EMPTY_ARG}"'));
  assert.ok(source.includes('bench_hot_group_activation_wait_ms="\\${14:-}"'));
  assert.ok(source.includes('local bench_script="${BENCH_SCRIPT:-$REMOTE_EMPTY_ARG}"'));
  assert.ok(source.includes('bench_env+=(BENCH_SCRIPT="\\$bench_script")'));
  assert.ok(source.includes('local bench_phone_prefix="${BENCH_PHONE_PREFIX:-$REMOTE_EMPTY_ARG}"'));
  assert.ok(source.includes('bench_env+=(PHONE_PREFIX="\\$bench_phone_prefix")'));
  assert.ok(source.includes('local bench_hot_group_member_count_threshold="${BENCH_HOT_GROUP_MEMBER_COUNT_THRESHOLD:-$REMOTE_EMPTY_ARG}"'));
  assert.ok(source.includes('bench_env+=(HOT_GROUP_MESSAGE_THRESHOLD="\\$bench_hot_group_message_threshold")'));
  assert.ok(source.includes('[[ "\\${!bench_arg}" == "${REMOTE_EMPTY_ARG}" ]]'));
});

test("candidate image builds are explicit and carry source provenance", () => {
  assert.match(source, /REMOTE_BUILD_CANDIDATE="\$\{DIPOLE_REMOTE_BUILD_CANDIDATE:-0\}"/);
  assert.match(source, /candidate_tag="dipole-server:c1-\\\$\(git rev-parse --short HEAD\)"/);
  assert.match(source, /--build-arg DIPOLE_VCS_REVISION="\\\$\{candidate_revision\}"/);
  assert.match(source, /--build-arg DIPOLE_BUILD_CREATED="\\\$\{candidate_created\}"/);
  assert.match(source, /--build-arg DIPOLE_VCS_DIRTY=false/);
  assert.match(source, /candidate_revision="\\\$\(git rev-parse HEAD\)"/);
});

test("remote node toolchain is explicit and version-gated", () => {
  assert.match(source, /REMOTE_NODE_ROOT="\$\{DIPOLE_REMOTE_NODE_ROOT:-/);
  assert.match(source, /required_node="v22\.0\.0"/);
  assert.match(source, /remote node-test refused: requires Node %s\+/);
});

test("remote Go toolchain prefers explicit configuration and discovers the newest user-local install", () => {
  assert.match(source, /if \[\[ -z "\\\$go_root" \]\]; then/);
  assert.match(source, /find \/home\/admin1\/\.local -maxdepth 4 -type f -path '\*\/bin\/go'/);
  assert.match(source, /sort -V/);
  assert.match(source, /remote Go toolchain auto-selected/);
  assert.match(source, /DIPOLE_REMOTE_GO_ROOT/);
});
