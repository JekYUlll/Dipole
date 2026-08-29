import { createHash } from "node:crypto";

import { describe, expect, it } from "vitest";

import { canonicalJSON } from "./offline-evaluator.js";
import { type SubscriptionRolloutDecision } from "./subscription-rollout-evaluator.js";
import { SubscriptionRuntimeGate } from "./subscription-runtime-gate.js";

describe("SubscriptionRuntimeGate", () => {
  it("bypasses the candidate in off mode", () => {
    const gate = new SubscriptionRuntimeGate({ mode: "off", decision: decision("blocked") });
    expect(gate.evaluate()).toMatchObject({ outcome: "bypassed", taskCreationAllowed: true });
  });

  it("observes an eligible candidate without changing task admission in shadow mode", () => {
    const gate = new SubscriptionRuntimeGate(binding("shadow", "eligible"));
    expect(gate.evaluate()).toMatchObject({ outcome: "observed", taskCreationAllowed: true });
  });

  it("admits only an eligible, exactly bound candidate in enforced mode", () => {
    const gate = new SubscriptionRuntimeGate(binding("enforced", "eligible"));
    expect(gate.evaluate()).toMatchObject({ outcome: "admitted", taskCreationAllowed: true });

    const blocked = new SubscriptionRuntimeGate(binding("enforced", "blocked"));
    expect(blocked.evaluate()).toMatchObject({ outcome: "blocked", taskCreationAllowed: false });
  });

  it("fails closed when the rollout decision binding drifts", () => {
    const candidate = decision("eligible");
    const gate = new SubscriptionRuntimeGate({
      mode: "enforced", decision: candidate,
      decisionSha256: digest({ ...candidate, decision: "blocked" })
    });
    expect(() => gate.evaluate()).toThrow(/rollout decision hash drift/iu);
  });
});

function binding(mode: "shadow" | "enforced", status: SubscriptionRolloutDecision["decision"]) {
  const candidate = decision(status);
  return {
    mode, decision: candidate, decisionSha256: digest(candidate), candidateId: candidate.candidate.id,
    configurationSha256: candidate.candidate.configurationSha256, corpusSha256: candidate.corpusSha256,
    reviewSha256: candidate.reviewSha256, finalLabelsSha256: candidate.finalLabelsSha256,
    candidateEvidenceSha256: candidate.candidateEvidenceSha256
  };
}

function decision(value: SubscriptionRolloutDecision["decision"]): SubscriptionRolloutDecision {
  return {
    schemaVersion: "dipole.agent.subscription-rollout-decision.v1", decision: value,
    reasons: value === "eligible" ? [] : ["candidate_prefilter_blocked"],
    corpusSha256: "a".repeat(64), reviewSha256: "b".repeat(64), finalLabelsSha256: "c".repeat(64),
    candidateEvidenceSha256: "d".repeat(64), candidate: {
      id: "embedding:v1", kind: "embedding", revision: "embed@1", configurationSha256: "e".repeat(64)
    }, metrics: { agreementBps: 10000, minimumAgreementBps: 9000, precisionBps: 9500, recallBps: 9500, p95LatencyMicros: 100, meanCostMicrousd: 1 }
  };
}

function digest(value: unknown): string { return createHash("sha256").update(canonicalJSON(value)).digest("hex"); }
