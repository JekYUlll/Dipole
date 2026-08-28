import { createHash } from "node:crypto";

import { describe, expect, it } from "vitest";

import { canonicalJSON } from "./offline-evaluator.js";
import { MemoryPrefilterRuntimeGate, type MemoryPrefilterRuntimeBinding } from "./memory-prefilter-runtime-gate.js";
import type { MemoryPrefilterRolloutDecision } from "./memory-prefilter-rollout.js";

const decision: MemoryPrefilterRolloutDecision = {
  schemaVersion: "dipole.agent.memory-prefilter-rollout-decision.v1", decision: "eligible", reasons: [],
  corpusSha256: "a".repeat(64), reviewSha256: "b".repeat(64), finalLabelsSha256: "c".repeat(64), candidateEvidenceSha256: "d".repeat(64),
  candidate: { id: "embedding:v1", kind: "embedding", revision: "model@1", configurationSha256: "e".repeat(64), decisionThresholdBps: 7_500 },
  metrics: { agreementBps: 10_000, minimumAgreementBps: 10_000, precisionBps: 10_000, recallBps: 10_000, p95LatencyMicros: 100, meanCostMicrousd: 1 }
};

function binding(mode: MemoryPrefilterRuntimeBinding["mode"] = "enforced", override: Partial<MemoryPrefilterRuntimeBinding> = {}): MemoryPrefilterRuntimeBinding {
  return { mode, decision, decisionSha256: digest(decision), candidateId: "embedding:v1", configurationSha256: "e".repeat(64), corpusSha256: decision.corpusSha256, reviewSha256: decision.reviewSha256, ...override };
}

describe("memory prefilter runtime gate", () => {
  it("admits only an eligible, exactly bound decision", () => {
    expect(new MemoryPrefilterRuntimeGate(binding()).evaluate()).toMatchObject({ outcome: "admitted", taskCreationAllowed: true, memoryWriteAuthority: false });
  });

  it("keeps off and shadow modes non-blocking without write authority", () => {
    expect(new MemoryPrefilterRuntimeGate(binding("off")).evaluate()).toMatchObject({ outcome: "bypassed", taskCreationAllowed: true });
    expect(new MemoryPrefilterRuntimeGate(binding("shadow")).evaluate()).toMatchObject({ outcome: "observed", taskCreationAllowed: true, memoryWriteAuthority: false });
  });

  it("blocks an ineligible decision and rejects binding drift", () => {
    const blocked = { ...decision, decision: "blocked" as const, reasons: ["candidate_prefilter_blocked"] as ["candidate_prefilter_blocked"] };
    expect(new MemoryPrefilterRuntimeGate(binding("enforced", { decision: blocked, decisionSha256: digest(blocked) })).evaluate()).toMatchObject({ outcome: "blocked", taskCreationAllowed: false });
    expect(() => new MemoryPrefilterRuntimeGate(binding("enforced", { decisionSha256: "f".repeat(64) })).evaluate()).toThrow(/hash drift/);
    expect(() => new MemoryPrefilterRuntimeGate(binding("enforced", { candidateId: "small-model:v1" })).evaluate()).toThrow(/candidate binding/);
  });
});

function digest(value: unknown): string { return createHash("sha256").update(canonicalJSON(value)).digest("hex"); }
