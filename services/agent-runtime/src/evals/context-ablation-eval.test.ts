import { describe, expect, it } from "vitest";
import { evaluateContextAblation, parseContextAblationEvalSuite } from "./context-ablation-eval.js";

describe("context ablation Eval", () => {
  it("compares baseline, retrieval and Memory without source content", () => {
    const report = evaluateContextAblation(parseContextAblationEvalSuite({ schemaVersion: "dipole.agent.context-ablation-eval.v1", candidateVersion: "agent@1", cases: [{ caseId: "case:1", requiredOutputIds: ["output:task"], relevantEvidenceIds: ["evidence:task"], results: [
      { condition: "baseline", outputIds: [], evidenceIds: [], allowed: true, metrics: { modelCalls: 1, toolCalls: 0, totalTokens: 10, totalCostMicrousd: 1, latencyMs: 10 } },
      { condition: "retrieval", outputIds: ["output:task"], evidenceIds: ["evidence:task"], allowed: true, metrics: { modelCalls: 1, toolCalls: 1, totalTokens: 20, totalCostMicrousd: 2, latencyMs: 20 } },
      { condition: "memory", outputIds: ["output:task"], evidenceIds: ["evidence:task"], allowed: true, metrics: { modelCalls: 1, toolCalls: 1, totalTokens: 15, totalCostMicrousd: 2, latencyMs: 15 } }
    ] }] }));
    expect(report.conditions).toMatchObject({ baseline: { completedCases: 0 }, retrieval: { completedCases: 1, evidenceRecallCases: 1 }, memory: { completedCases: 1, metrics: { totalTokens: 15 } } });
    expect(JSON.stringify(report)).not.toContain("conversation body");
  });
});
