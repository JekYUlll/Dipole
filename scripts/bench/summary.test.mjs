import assert from "node:assert/strict";
import { test } from "node:test";
import { handleSummary } from "./summary.js";

test("exports numeric metrics without setup credentials or arbitrary metadata", () => {
  globalThis.__ENV = { SUMMARY_JSON: "report.json" };
  const input = {
    setup_data: { users: [{ token: "private-token", password: "private-password" }] },
    root_group: { name: "private-token" },
    metrics: { latency: { type: "trend", values: { "p(95)": 12 }, thresholds: { "p(95)<20": { ok: true } } } }
  };
  const output = handleSummary(input);
  assert.deepEqual(JSON.parse(output["report.json"]), {
    metrics: { latency: { "p(95)": 12, thresholds: { "p(95)<20": true } } }
  });
  assert.equal(output["report.json"].includes("private"), false);
  assert.equal(input.setup_data.users[0].token, "private-token");
});

test("direct execution prints only the safe report", () => {
  globalThis.__ENV = {};
  assert.deepEqual(JSON.parse(handleSummary({ setup_data: { token: "secret" } }).stdout), { metrics: {} });
});
