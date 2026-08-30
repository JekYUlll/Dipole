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

test("remote image builds compile committed backend binaries first", () => {
  assert.match(source, /build\)[\s\S]*?scripts\/docker-build\.sh backend && scripts\/docker-build-microservice-images\.sh/);
});

test("remote node toolchain is explicit and version-gated", () => {
  assert.match(source, /REMOTE_NODE_ROOT="\$\{DIPOLE_REMOTE_NODE_ROOT:-/);
  assert.match(source, /required_node="v22\.0\.0"/);
  assert.match(source, /remote node-test refused: requires Node %s\+/);
});
