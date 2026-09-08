import { describe, expect, it } from "vitest";

import { evaluateMemoryB1SyntheticEval, parseMemoryB1SyntheticEvalSuite } from "./memory-b1-synthetic-eval.js";

const hash = (value: string) => value.repeat(64).slice(0, 64);
const fixture = () => ({
  schemaVersion: "dipole.agent.memory-b1-synthetic-eval.v1", candidateVersion: "agent-runtime@b1-fixture", minimumPassBps: 10_000,
  cases: [
    { caseId: "recall-orbit", canarySha256: hash("a"), expectedMemoryLineageCount: 1, recallExpectation: "required" },
    { caseId: "revoke-orbit", canarySha256: hash("a"), expectedMemoryLineageCount: 0, recallExpectation: "not_evaluated" }
  ],
  observations: [
    { caseId: "recall-orbit", canarySha256: hash("a"), taskSha256: hash("b"), replySha256: hash("c"), taskCompleted: true, modelCallCount: 1, memoryLineageCount: 1, responseContainsCanary: true },
    { caseId: "revoke-orbit", canarySha256: hash("a"), taskSha256: hash("d"), replySha256: hash("e"), taskCompleted: true, modelCallCount: 1, memoryLineageCount: 0, responseContainsCanary: true }
  ]
});

describe("B1 synthetic Memory Eval", () => {
  it("reports recall and revoke evidence without source text", () => {
    const report = evaluateMemoryB1SyntheticEval(fixture());
    expect(report).toMatchObject({ passed: true, metrics: { totalCases: 2, recallCases: 1, revokedCases: 1, passBps: 10_000 } });
    expect(JSON.stringify(report)).not.toContain("ORBIT-91");
  });

  it("fails closed on recall, Task, lineage, and duplicate binding drift", () => {
    expect(evaluateMemoryB1SyntheticEval({ ...fixture(), observations: [{ ...fixture().observations[0], responseContainsCanary: false }, fixture().observations[1]] }).failures[0]).toMatchObject({ reasons: ["synthetic_canary_not_recalled"] });
    expect(evaluateMemoryB1SyntheticEval({ ...fixture(), observations: [{ ...fixture().observations[0], modelCallCount: 2 }, fixture().observations[1]] }).failures[0]).toMatchObject({ reasons: ["model_call_count_mismatch"] });
    expect(() => parseMemoryB1SyntheticEvalSuite({ ...fixture(), observations: [fixture().observations[0], { ...fixture().observations[1], taskSha256: hash("b") }] })).toThrow(/Task hashes/i);
  });
});
