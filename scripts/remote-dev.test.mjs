import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

const source = fs.readFileSync(new URL("./remote-dev.sh", import.meta.url), "utf8");

test("remote sync only accepts a clean committed worktree", () => {
  assert.match(source, /sync_revision\(\)[\s\S]*?git status --porcelain/);
  assert.match(source, /commit or stash local changes before remote sync/);
});

test("node verification preserves package locks and cleans generated webapp output", () => {
  assert.match(source, /node-test\)[\s\S]*?--package-lock=false/);
  assert.match(source, /webapp_dir="internal\/services\/core\/server\/webapp"/);
  assert.match(source, /generated webapp output is dirty/);
  assert.match(source, /trap cleanup_webapp EXIT/);
});

test("remote destructive actions remain behind the active-host guard", () => {
  assert.match(source, /build\) sync_revision; guard_start; run_remote build/);
  assert.match(source, /smoke-lite\) sync_revision; guard_start; run_remote smoke-lite/);
  assert.match(source, /bench\) sync_revision; guard_start; run_remote bench/);
});

test("multipart smoke is isolated and does not require a GPU-free host", () => {
  assert.match(source, /multipart-smoke\) sync_revision; run_remote multipart-smoke/);
  assert.match(source, /multipart-smoke\)[\s\S]*?GOTOOLCHAIN=local scripts\/smoke-minio-multipart\.sh/);
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
