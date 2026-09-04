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
    env: {
      ...process.env,
      DIPOLE_INTERNAL_RPC_SHARED_SECRET: "test-only",
      DIPOLE_AGENT_MODEL_API_KEY: "test-only",
      DIPOLE_AGENT_MODEL_BASE_URL: "https://example.test/v1",
      DIPOLE_AGENT_MODEL_CONTEXT_PROFILES: "{}",
      DIPOLE_AGENT_MODEL_PROVIDER_NAME: "test-provider",
      DIPOLE_AGENT_MODEL_ROUTES: "{}",
    },
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

test("web sync observability smoke stays isolated and does not claim promotion", () => {
  const script = execFileSync("cat", [path.join(root, "scripts/smoke-web-sync-observability.sh")], {
    encoding: "utf8",
  });

  for (const required of [
    "--profile observability up -d --wait gateway prometheus alertmanager",
    "DIPOLE_WEB_SYNC_OBSERVABILITY_STARTUP_TIMEOUT_SECONDS:-300",
    "timeout --preserve-status \"${startup_timeout_seconds}s\" docker compose",
    "--profile observability down -v --remove-orphans",
    "wait_for_healthy_targets",
    "required_targets_are_healthy",
    "http://127.0.0.1:9100/metrics",
    "DIPOLE_PROMETHEUS_PORT:-9090",
    "DIPOLE_ALERTMANAGER_PORT:-9093",
    "${ALERTMANAGER_URL}/-/ready",
    'required = {"dipole-core", "dipole-message", "dipole-sync", "dipole-gateway"}',
    "api/v1/targets?state=active&scrapePool=dipole-required",
    "does not start a Web Sync promotion observation window",
  ]) {
    assert.match(script, new RegExp(required.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  }
});
