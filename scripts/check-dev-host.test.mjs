import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const script = path.join(root, "scripts", "check-dev-host.sh");

function run(profile, overrides = {}) {
  try {
    return { status: 0, output: execFileSync(script, [profile], {
      cwd: root,
      env: { ...process.env, DIPOLE_SKIP_DOCKER: "1", DIPOLE_SKIP_COMPOSE: "1", ...overrides },
      encoding: "utf8",
    }) };
  } catch (error) {
    return { status: error.status, output: `${error.stdout ?? ""}${error.stderr ?? ""}` };
  }
}

test("accepts the Remote GPU development profile", () => {
  const result = run("remote-gpu", {
    DIPOLE_HOST_CPU: "224",
    DIPOLE_HOST_MEMORY_MIB: "192512",
    DIPOLE_HOST_DISK_MIB: "1100000",
  });
  assert.equal(result.status, 0);
  assert.match(result.output, /PASS:.*remote-gpu/);
});

test("accepts TencentCloud for lightweight development checks", () => {
  const result = run("tencent-cloud", {
    DIPOLE_HOST_CPU: "2",
    DIPOLE_HOST_MEMORY_MIB: "1100",
    DIPOLE_HOST_DISK_MIB: "34000",
  });
  assert.equal(result.status, 0);
});

test("rejects the local host when it cannot meet the full profile", () => {
  const result = run("local", {
    DIPOLE_HOST_CPU: "16",
    DIPOLE_HOST_MEMORY_MIB: "27000",
    DIPOLE_HOST_DISK_MIB: "19000",
  });
  assert.equal(result.status, 1);
  assert.match(result.output, /disk_mib=19000/);
});

test("uses the available-memory override for pressure-aware checks", () => {
  const result = run("remote-gpu", {
    DIPOLE_HOST_CPU: "224",
    DIPOLE_HOST_MEMORY_MIB: "8192",
    DIPOLE_HOST_DISK_MIB: "1100000",
  });
  assert.equal(result.status, 1);
  assert.match(result.output, /memory_mib=8192/);
});

test("rejects an unknown profile", () => {
  const result = run("unknown");
  assert.equal(result.status, 2);
});
