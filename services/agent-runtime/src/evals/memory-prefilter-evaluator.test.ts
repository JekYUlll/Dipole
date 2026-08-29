import { describe, expect, it } from "vitest";
import { createHash } from "node:crypto";

import {
  evaluateMemoryPrefilter,
  parseMemoryPrefilterEvidence,
  type MemoryPrefilterEvidence,
  type MemoryPrefilterPolicy
} from "./memory-prefilter-evaluator.js";
import { parseMemoryReviewedCorpus } from "./memory-reviewed-corpus.js";
import { canonicalJSON } from "./offline-evaluator.js";

const corpusRecord = {
  schemaVersion: "dipole.agent.memory-reviewed-corpus.v1",
  corpusId: "memory-gold",
  cases: [
    { caseId: "case-a", candidateType: "message", resourceType: "conversation", evidenceCount: 2, contentSha256: "a".repeat(64), goldPromotable: true },
    { caseId: "case-b", candidateType: "reflection", resourceType: "project", evidenceCount: 3, contentSha256: "b".repeat(64), goldPromotable: false },
    { caseId: "case-c", candidateType: "message", resourceType: "conversation", evidenceCount: 1, contentSha256: "c".repeat(64), goldPromotable: true },
    { caseId: "case-d", candidateType: "message", resourceType: "conversation", evidenceCount: 1, contentSha256: "d".repeat(64), goldPromotable: false }
  ],
};
const corpus = parseMemoryReviewedCorpus({
  ...corpusRecord,
  sha256: createHash("sha256").update(canonicalJSON(corpusRecord)).digest("hex")
});

const policy: MemoryPrefilterPolicy = {
  minimumPrecisionBps: 10_000,
  minimumRecallBps: 10_000,
  maximumP95LatencyMicros: 2_000,
  maximumMeanCostMicrousd: 20
};

function evidence(overrides: Partial<MemoryPrefilterEvidence> = {}): MemoryPrefilterEvidence {
  return {
    schemaVersion: "dipole.agent.memory-prefilter-evidence.v1",
    corpusSha256: corpus.sha256,
    candidate: {
      id: "embedding:v1",
      kind: "embedding",
      revision: "model@1",
      configurationSha256: "e".repeat(64),
      decisionThresholdBps: 7_500
    },
    decisions: [
      { caseId: "case-a", selected: true, scoreBps: 9_000, latencyMicros: 1_000, costMicrousd: 10 },
      { caseId: "case-b", selected: false, scoreBps: 1_000, latencyMicros: 1_100, costMicrousd: 10 },
      { caseId: "case-c", selected: true, scoreBps: 8_000, latencyMicros: 1_200, costMicrousd: 10 },
      { caseId: "case-d", selected: false, scoreBps: 2_000, latencyMicros: 1_300, costMicrousd: 10 }
    ],
    ...overrides
  };
}

describe("memory prefilter evaluator", () => {
  it("evaluates embedding evidence against reviewed gold labels", () => {
    const report = evaluateMemoryPrefilter(corpus, evidence(), policy);
    expect(report).toMatchObject({ passed: true, confusion: { truePositive: 2, trueNegative: 2, falsePositive: 0, falseNegative: 0 } });
    expect(report.metrics).toMatchObject({ precisionBps: 10_000, recallBps: 10_000, p95LatencyMicros: 1_300, meanCostMicrousd: 10 });
    expect(JSON.stringify(report)).not.toContain("case-a");
  });

  it("fails closed when a scored decision conflicts with its threshold", () => {
    expect(() => parseMemoryPrefilterEvidence({ ...evidence(), decisions: [{ ...evidence().decisions[0], selected: false }] })).toThrow(/threshold/);
  });

  it("rejects incomplete case coverage and corpus drift", () => {
    expect(() => evaluateMemoryPrefilter(corpus, evidence({ decisions: evidence().decisions.slice(1) }), policy)).toThrow(/exactly one decision/);
    expect(() => evaluateMemoryPrefilter(corpus, evidence({ corpusSha256: "f".repeat(64) }), policy)).toThrow(/SHA-256/);
  });

  it("reports gate failures without exposing corpus identifiers", () => {
    const report = evaluateMemoryPrefilter(corpus, evidence({ decisions: evidence().decisions.map(item => ({ ...item, selected: true, scoreBps: 9_000 })) }), { ...policy, minimumPrecisionBps: 10_000 });
    expect(report).toMatchObject({ passed: false, reasons: ["precision_below_minimum"] });
    expect(report).not.toHaveProperty("falsePositiveCaseIds");
    expect(report).not.toHaveProperty("falseNegativeCaseIds");
  });
});
