import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

test("lightweight smoke targets only the gateway dependency closure", () => {
  const raw = execFileSync("docker", [
    "compose", "-f", path.join(root, "deploy/compose/docker-compose.microservices.yml"),
    "config", "--format", "json",
  ], {
    cwd: root,
    env: { ...process.env, DIPOLE_INTERNAL_RPC_SHARED_SECRET: "test-only" },
    encoding: "utf8",
  });
  const services = JSON.parse(raw).services;
  const closure = new Set();
  const visit = (name) => {
    if (closure.has(name)) return;
    closure.add(name);
    for (const dependency of Object.keys(services[name]?.depends_on ?? {})) visit(dependency);
  };
  visit("gateway");

  assert.deepEqual([...closure].sort(), [
    "core", "gateway", "kafka", "message", "minio", "minio-init",
    "migrate", "mysql", "mysql-permissions", "redis", "sync",
  ].sort());
  for (const optional of ["agent", "search", "search-indexer", "realtime-cpp"]) {
    assert.equal(closure.has(optional), false, `${optional} must stay outside the smoke closure`);
  }
});
