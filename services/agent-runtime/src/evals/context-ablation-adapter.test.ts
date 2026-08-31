import { describe, expect, it } from "vitest";

import { buildContextAblationEvalSuite } from "./context-ablation-adapter.js";
import type { ContextAblationCaseObservation } from "./mysql-context-ablation-store.js";

const sha = "a".repeat(64);
const evidence = "evidence:a65b06572d9e1a55e5172e62fd8fa7b3";
const manifest = {
  schemaVersion: "dipole.agent.context-ablation-manifest.v1",
  experimentId: "experiment:1",
  candidateVersion: "agent@1",
  routePrices: [{ route: "deepseek/flash", inputMicrousdPerMillionTokens: 1_000_000, outputMicrousdPerMillionTokens: 2_000_000 }],
  cases: [{ caseSha256: sha, requiredOutputIds: ["artifact:conversation_digest:v1"], relevantEvidenceIds: [evidence] }]
} as const;

function observation(): ContextAblationCaseObservation {
  const item = {
    taskId: "task:1", taskStatus: "completed", runId: "run:1", runStatus: "completed", traceId: "trace:1",
    contextManifest: { selected: [{ id: "selected:1", provenance: { sourceType: "conversation", sourceId: "source:1" } }], omitted: [] },
    steps: [{ stepNo: 1, capabilityId: "conversation.read", status: "completed", attemptCount: 1, latencyMs: 3, authorization: { resourceType: "conversation", resourceId: "conversation:1", action: "read", decision: "allowed" as const } }],
    artifacts: [{ artifactType: "conversation_digest", version: 1 }],
    modelCalls: [{ route: "deepseek/flash", status: "completed", inputTokens: 2, outputTokens: 3, latencyMs: 4 }],
    toolCalls: [{ status: "completed", latencyMs: 5 }]
  };
  return { caseSha256: sha, candidateVersion: "agent@1", observations: { baseline: item, retrieval: item, memory: item } };
}

describe("Context Ablation observation adapter", () => {
  it("compiles reviewed labels and sanitized observations without source content", () => {
    const suite = buildContextAblationEvalSuite(manifest, [observation()]);
    expect(suite.cases[0]).toMatchObject({ caseId: `case:${sha}`, results: [{ condition: "baseline", outputIds: ["artifact:conversation_digest:v1"], evidenceIds: [evidence], allowed: true, metrics: { totalTokens: 5, totalCostMicrousd: 8, latencyMs: 12 } }] });
    expect(JSON.stringify(suite)).not.toContain("source:1");
  });

  it("rejects incomplete token metering before a report can be evaluated", () => {
    const item = observation();
    const broken = { ...item, observations: { ...item.observations, memory: { ...item.observations.memory, modelCalls: [{ ...item.observations.memory.modelCalls[0]!, inputTokens: null }] } } };
    expect(() => buildContextAblationEvalSuite(manifest, [broken])).toThrow("Context ablation model call lacks complete metering");
  });

  it("rejects a candidate version that differs from the reviewed manifest", () => {
    expect(() => buildContextAblationEvalSuite(manifest, [{ ...observation(), candidateVersion: "agent@2" }])).toThrow("Context ablation candidate version does not match the manifest");
  });
});
