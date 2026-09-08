import { describe, expect, it } from "vitest";

import { evaluateMemoryB1SyntheticWindow, parseMemoryB1SyntheticWindow } from "./memory-b1-synthetic-window-eval.js";

const hash = (value: string) => value.repeat(64).slice(0, 64);
const suite = (canary: string, recalled: boolean) => ({
  schemaVersion: "dipole.agent.memory-b1-synthetic-eval.v1", candidateVersion: "agent-runtime@window-fixture", minimumPassBps: 10_000,
  cases: [
    { caseId: "recall", canarySha256: hash(canary), expectedMemoryLineageCount: 1, recallExpectation: "required" },
    { caseId: "revoke", canarySha256: hash(canary), expectedMemoryLineageCount: 0, recallExpectation: "not_evaluated" }
  ],
  observations: [
    { caseId: "recall", canarySha256: hash(canary), taskSha256: hash(`${canary}a`), replySha256: hash(`${canary}b`), taskCompleted: true, modelCallCount: 1, memoryLineageCount: 1, responseContainsCanary: recalled },
    { caseId: "revoke", canarySha256: hash(canary), taskSha256: hash(`${canary}c`), replySha256: hash(`${canary}d`), taskCompleted: true, modelCallCount: 1, memoryLineageCount: 0, responseContainsCanary: false }
  ]
});

describe("B1 synthetic Memory window", () => {
  it("preserves a valid failed recall window", () => {
    const report = evaluateMemoryB1SyntheticWindow({
      schemaVersion: "dipole.agent.memory-b1-synthetic-window.v1", candidateVersion: "agent-runtime@window-fixture", minimumRecallPassBps: 10_000,
      suites: [suite("a", true), suite("b", true), suite("c", false)]
    });
    expect(report).toMatchObject({ passed: false, reasons: ["recall_below_minimum"], metrics: { recallCases: 3, recalledCases: 2, recallPassBps: 6666, revokeCases: 3, revokeBoundaryFailures: 0, invariantFailures: 0 } });
    expect(report.failedRecallCanarySha256).toEqual([hash("c")]);
  });

  it("fails closed on candidate drift and duplicate recall canaries", () => {
    const window = { schemaVersion: "dipole.agent.memory-b1-synthetic-window.v1", candidateVersion: "agent-runtime@window-fixture", minimumRecallPassBps: 0, suites: [suite("a", true), suite("b", true), suite("c", true)] };
    expect(() => parseMemoryB1SyntheticWindow({ ...window, suites: [suite("a", true), suite("a", true), suite("c", true)] })).toThrow(/canaries/i);
    expect(() => parseMemoryB1SyntheticWindow({ ...window, suites: [{ ...suite("a", true), candidateVersion: "agent-runtime@other" }, suite("b", true), suite("c", true)] })).toThrow(/candidate version/i);
  });
});
