import { describe, expect, it } from "vitest";

import {
  createCoreRestartReadShadowEvidence,
  parseCoreRestartReadShadowEvidence
} from "./core-restart-read-shadow-evidence.js";

const now = new Date("2026-08-31T12:00:00.000Z");
const outcome = {
  event_id: "SMOKE-AGENT-EVENT-1",
  core_restart_triggered: true,
  core_ready_after_restart: true,
  gateway_proxy_recovered: true,
  ledger_completed_event_count: 1,
  task_count: 1,
  completed_run_count: 1,
  completed_model_call_count: 1,
  conversation_digest_artifact_count: 1
} as const;

describe("Core restart read-shadow evidence", () => {
  it("binds a disposable Core restart drill to the exact read-shadow convergence counts", () => {
    const evidence = createCoreRestartReadShadowEvidence(outcome, { now: () => now });
    expect(evidence).toMatchObject({ outcome: "passed", production_authority: false, content_sha256: expect.stringMatching(/^[a-f0-9]{64}$/) });
    expect(parseCoreRestartReadShadowEvidence(evidence, { now: () => now })).toEqual(evidence);
  });

  it("rejects a changed event count, authority or content binding", () => {
    const evidence = createCoreRestartReadShadowEvidence(outcome, { now: () => now });
    expect(() => parseCoreRestartReadShadowEvidence({ ...evidence, task_count: 2 }, { now: () => now })).toThrow();
    expect(() => parseCoreRestartReadShadowEvidence({ ...evidence, production_authority: true }, { now: () => now })).toThrow();
    expect(() => parseCoreRestartReadShadowEvidence({ ...evidence, content_sha256: "0".repeat(64) }, { now: () => now })).toThrow();
  });

  it("rejects expired evidence", () => {
    const evidence = createCoreRestartReadShadowEvidence(outcome, { now: () => now, validityMs: 1_000 });
    expect(() => parseCoreRestartReadShadowEvidence(evidence, { now: () => new Date("2026-08-31T12:00:02.000Z") })).toThrow(/invalid/i);
  });
});
