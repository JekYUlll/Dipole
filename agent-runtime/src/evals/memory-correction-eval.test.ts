import { describe, expect, it } from "vitest";

import { evaluateOfflineEvalSuite } from "./offline-evaluator.js";
import { buildMemoryCorrectionEvalSuite, parseMemoryCorrectionEvalManifest, parseMemoryCorrectionEvalObservation } from "./memory-correction-eval.js";

describe("Agent Memory correction Eval", () => {
  it("builds a low-sensitive five-category suite from canonical correction evidence", () => {
    const manifest = parseMemoryCorrectionEvalManifest(manifestFixture());
    const observation = parseMemoryCorrectionEvalObservation(observationFixture());
    const suite = buildMemoryCorrectionEvalSuite(manifest, observation);
    const report = evaluateOfflineEvalSuite(suite);

    expect(report).toMatchObject({ passed: true, summary: { total: 5, passed: 5 } });
    expect(report.summary.categories).toEqual({
      outcome: { total: 1, passed: 1 }, trajectory: { total: 1, passed: 1 }, permission: { total: 1, passed: 1 },
      retrieval: { total: 1, passed: 1 }, cost: { total: 1, passed: 1 }
    });
    const serialized = JSON.stringify(suite);
    expect(serialized).not.toContain("MEM-ROOT-PRIVATE");
    expect(serialized).not.toContain("U100");
    expect(serialized).not.toContain("Owner is Bob");
  });

  it("fails closed on lineage, replay, permission and retrieval drift", () => {
    const manifest = parseMemoryCorrectionEvalManifest(manifestFixture());
    const base = observationFixture();

    expect(() => buildMemoryCorrectionEvalSuite(manifest, parseMemoryCorrectionEvalObservation({
      ...base, corrected: { ...base.corrected, supersedesMemoryId: "MEM-OTHER" }
    }))).toThrow(/predecessor/i);
    expect(() => buildMemoryCorrectionEvalSuite(manifest, parseMemoryCorrectionEvalObservation({
      ...base, exactReplayConverged: false
    }))).toThrow(/replay/i);
    expect(() => buildMemoryCorrectionEvalSuite(manifest, parseMemoryCorrectionEvalObservation({
      ...base, authorizationChecks: [{ actorRole: "owner", decision: "allowed" }]
    }))).toThrow(/permission/i);
    expect(() => buildMemoryCorrectionEvalSuite(manifest, parseMemoryCorrectionEvalObservation({
      ...base, retrievedMemoryIds: ["MEM-SOURCE"]
    }))).toThrow(/retrieval/i);
  });

  it("rejects unknown fields, content-bearing evidence and nonzero model or Tool use", () => {
    expect(() => parseMemoryCorrectionEvalManifest({ ...manifestFixture(), unexpected: true })).toThrow();
    expect(() => parseMemoryCorrectionEvalObservation({ ...observationFixture(), content: "Owner is Bob" })).toThrow();
    expect(() => parseMemoryCorrectionEvalObservation({
      ...observationFixture(), metrics: { modelCalls: 1, toolCalls: 0, totalTokens: 0, totalCostMicrousd: 0, latencyMs: 80 }
    })).toThrow(/zero-model/i);
  });
});

function manifestFixture() {
  return {
    schemaVersion: "dipole.agent.memory-correction-eval-manifest.v1",
    candidateVersion: "agent-memory@correction-v1",
    expectedSourceVersion: 1,
    expectedTrajectory: ["memory.owner.get", "memory.owner.lock", "memory.predecessor.revoke", "memory.successor.append", "memory.authority.read"],
    maximumLatencyMs: 500
  };
}

function observationFixture() {
  return {
    schemaVersion: "dipole.agent.memory-correction-eval-observation.v1",
    memoryRootId: "MEM-ROOT-PRIVATE",
    previous: { memoryId: "MEM-SOURCE", memoryVersion: 1, status: "revoked", revokedByRole: "owner" },
    corrected: {
      memoryId: "MEM-CORRECTED", memoryVersion: 2, status: "active", supersedesMemoryId: "MEM-SOURCE",
      correctedByRole: "owner", provenanceSourceType: "owner_correction", provenanceSourceId: "MEM-SOURCE", provenanceSequence: "2"
    },
    trajectory: ["memory.owner.get", "memory.owner.lock", "memory.predecessor.revoke", "memory.successor.append", "memory.authority.read"],
    authorizationChecks: [{ actorRole: "owner", decision: "allowed" }, { actorRole: "foreign", decision: "denied" }],
    retrievedMemoryIds: ["MEM-CORRECTED"], activeRootRecordCount: 1, lineageRecordCount: 2,
    exactReplayConverged: true, driftConflict: true,
    metrics: { modelCalls: 0, toolCalls: 0, totalTokens: 0, totalCostMicrousd: 0, latencyMs: 80 }
  };
}
