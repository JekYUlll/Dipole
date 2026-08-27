import { createHash } from "node:crypto";
import { describe, expect, it, vi } from "vitest";

import type { AgentArtifactCreateInput, AgentArtifactRecord } from "../capabilities/agent-capability-rpc.js";
import { evaluateOfflineEvalSuite, parseOfflineEvalSuite } from "../evals/offline-evaluator.js";
import type { AgentShadowPromotionEvidenceV2 } from "./agent-shadow-promotion-policy.js";
import { PromotionEvidencePublisher } from "./promotion-evidence-publisher.js";

describe("PromotionEvidencePublisher", () => {
  it("publishes canonical eligible v2 evidence and returns a low-sensitivity receipt", async () => {
    const evidence = eligibleEvidence();
    const createArtifact = vi.fn(async (input: AgentArtifactCreateInput): Promise<AgentArtifactRecord> => artifactRecord(input));
    const publisher = new PromotionEvidencePublisher({ createArtifact });

    const receipt = await publisher.publish({
      schemaVersion: "dipole.agent.promotion-evidence-publication.v1",
      tenantId: "TENANT-A", taskId: "TASK-1", runId: "RUN-1", runtimeId: "dipole-agent",
      definitionId: "DEF-1", definitionVersion: 7, evidence, requestId: "REQ-1", traceId: "TRACE-1"
    });

    const request = createArtifact.mock.calls[0]![0];
    expect(request).toMatchObject({
      tenantId: "TENANT-A", taskId: "TASK-1", runId: "RUN-1", artifactType: "promotion_evaluation", version: 1,
      title: "Agent Runtime promotion evaluation", mediaType: "application/json",
      metadata: {
        runtimeId: "dipole-agent", candidateVersion: "agent-runtime@abc1234", definitionId: "DEF-1",
        definitionVersion: 7, evalSuiteSHA256: evidence.offlineEvalReport.suiteSha256
      }, requestId: "REQ-1", traceId: "TRACE-1"
    });
    const body = JSON.parse(Buffer.from(request.content).toString("utf8"));
    expect(body).toMatchObject({
      schemaVersion: "dipole.agent.promotion-evaluation.v1", runtimeId: "dipole-agent",
      candidateVersion: "agent-runtime@abc1234", definition: { id: "DEF-1", version: 7 },
      evidence: { schemaVersion: "dipole.agent.shadow-promotion-evidence.v2" },
      decision: { schemaVersion: "dipole.agent.shadow-promotion-decision.v2", decision: "eligible" }
    });
    expect(Buffer.from(request.content).toString("utf8")).toBe(canonicalJSON(body));
    expect(receipt).toEqual({
      schemaVersion: "dipole.agent.promotion-evidence-receipt.v1", artifactId: expect.stringMatching(/^[a-f0-9]{64}$/),
      evidenceSHA256: createHash("sha256").update(request.content).digest("hex"), evalSuiteSHA256: evidence.offlineEvalReport.suiteSha256,
      tenantId: "TENANT-A", taskId: "TASK-1", runId: "RUN-1", runtimeId: "dipole-agent",
      candidateVersion: "agent-runtime@abc1234", definitionId: "DEF-1", definitionVersion: 7
    });
  });

  it("fails closed before Artifact RPC for blocked evidence or invalid bindings", async () => {
    const createArtifact = vi.fn();
    const publisher = new PromotionEvidencePublisher({ createArtifact });
    const evidence = eligibleEvidence();
    evidence.projectionEvals = { passed: 5, total: 6 };
    await expect(publisher.publish({
      schemaVersion: "dipole.agent.promotion-evidence-publication.v1",
      tenantId: "TENANT-A", taskId: "TASK-1", runId: "RUN-1", runtimeId: "dipole-agent",
      definitionId: "DEF-1", definitionVersion: 7, evidence
    })).rejects.toThrow(/eligible/);
    expect(createArtifact).not.toHaveBeenCalled();
  });
});

function eligibleEvidence(): AgentShadowPromotionEvidenceV2 {
  const suite = parseOfflineEvalSuite({
    schemaVersion: "dipole.agent.offline-eval-suite.v1", candidateVersion: "agent-runtime@abc1234",
    cases: [
      { id: "outcome.case", category: "outcome", expected: { requiredOutputIds: ["output.ok"], forbiddenOutputIds: [] }, observed: { outputIds: ["output.ok"] } },
      { id: "trajectory.case", category: "trajectory", expected: { steps: ["step.ok"], forbiddenSteps: [] }, observed: { steps: ["step.ok"] } },
      { id: "permission.case", category: "permission", expected: { decisions: [] }, observed: { decisions: [] } },
      { id: "retrieval.case", category: "retrieval", expected: { relevantEvidenceIds: ["evidence.ok"], minimumRecall: 1, minimumPrecision: 1 }, observed: { retrievedEvidenceIds: ["evidence.ok"] } },
      { id: "cost.case", category: "cost", expected: { maximums: { modelCalls: 1, toolCalls: 1, totalTokens: 10, totalCostMicrousd: 10, latencyMs: 10 } }, observed: { modelCalls: 1, toolCalls: 1, totalTokens: 10, totalCostMicrousd: 10, latencyMs: 10 } }
    ]
  });
  const started = Date.parse("2026-08-27T00:00:00.000Z");
  return {
    schemaVersion: "dipole.agent.shadow-promotion-evidence.v2", candidateVersion: suite.candidateVersion,
    windowStartedAt: new Date(started).toISOString(), windowEndedAt: new Date(started + 24 * 60 * 60 * 1000).toISOString(),
    observations: Array.from({ length: 25 }, (_, index) => ({
      candidateVersion: suite.candidateVersion, observedAt: new Date(started + index * 60 * 60 * 1000).toISOString(),
      report: { schemaVersion: "dipole.agent.projection-reconcile.v1", consistent: true, scanned: 5, outcomes: { match: 5, missing: 0, stale: 0, ahead: 0, conflict: 0, unavailable: 0 }, examples: [] }
    })),
    projectionEvals: { passed: 6, total: 6 }, offlineEvalReport: evaluateOfflineEvalSuite(suite)
  };
}

function artifactRecord(input: AgentArtifactCreateInput): AgentArtifactRecord {
  const contentSha256 = createHash("sha256").update(input.content).digest("hex");
  const artifactId = createHash("sha256").update(["dipole.agent.artifact.v1", input.taskId, input.runId, input.artifactType, String(input.version), contentSha256].join("\n")).digest("hex");
  return { schemaVersion: "dipole.agent.artifact.v1", artifactId, taskId: input.taskId, runId: input.runId, artifactType: input.artifactType,
    version: input.version, title: input.title, mediaType: input.mediaType, contentSha256, sizeBytes: input.content.byteLength, metadata: input.metadata };
}

function canonicalJSON(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(canonicalJSON).join(",")}]`;
  if (value !== null && typeof value === "object") return `{${Object.entries(value).sort(([a], [b]) => a.localeCompare(b)).map(([key, item]) => `${JSON.stringify(key)}:${canonicalJSON(item)}`).join(",")}}`;
  return JSON.stringify(value);
}
