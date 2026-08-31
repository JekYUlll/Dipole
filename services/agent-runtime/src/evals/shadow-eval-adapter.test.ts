import { readFile } from "node:fs/promises";

import { describe, expect, it } from "vitest";

import {
  buildShadowEvalSuite,
  parseShadowEvalManifest,
  type ShadowEvalObservation
} from "./shadow-eval-adapter.js";

describe("Shadow evaluation adapter", () => {
  it("parses the language-neutral example manifest", async () => {
    const source = await readFile(new URL("../../../../contracts/agent-evals/v1/shadow-manifest.example.json", import.meta.url), "utf8");
    expect(parseShadowEvalManifest(source)).toMatchObject({ schemaVersion: "dipole.agent.shadow-eval-manifest.v1" });
  });

  it("builds all five offline categories from persisted observations and reviewer labels", () => {
    const manifest = parseShadowEvalManifest({
      schemaVersion: "dipole.agent.shadow-eval-manifest.v1",
      candidateVersion: "agent-runtime@shadow-42",
      taskId: "TASK-42",
      runId: "RUN-42",
      labels: {
        outcome: { requiredOutputIds: ["artifact:conversation_digest:v1"], forbiddenOutputIds: ["message:unexpected"] },
        trajectory: {
          steps: ["context.compile", "capability:conversation.list:completed", "artifact:conversation_digest:v1"],
          forbiddenSteps: ["capability:message.system.send:completed"]
        },
        permission: [{
          stepNo: 1, capabilityId: "conversation.list", resourceType: "conversation", resourceId: "user:u100",
          action: "read", decision: "allowed"
        }],
        retrieval: {
          relevantEvidenceIds: ["evidence:9ca5a7ab8595d195421b6f96f544b8fb"], minimumRecall: 1, minimumPrecision: 1
        },
        cost: {
          maximums: { modelCalls: 1, toolCalls: 2, totalTokens: 300, totalCostMicrousd: 1000, latencyMs: 1000 },
          routePrices: [{ route: "gateway/primary", inputMicrousdPerMillionTokens: 2_000_000, outputMicrousdPerMillionTokens: 6_000_000 }]
        }
      }
    });

    const suite = buildShadowEvalSuite(manifest, observation());

    expect(suite).toEqual({
      schemaVersion: "dipole.agent.offline-eval-suite.v1",
      candidateVersion: "agent-runtime@shadow-42",
      cases: [{
        id: "outcome.shadow.e2d3583d196abdf176eaff43", category: "outcome",
        expected: manifest.labels.outcome,
        observed: { outputIds: ["artifact:conversation_digest:v1", "run:completed", "task:completed"] }
      }, {
        id: "trajectory.shadow.e2d3583d196abdf176eaff43", category: "trajectory",
        expected: manifest.labels.trajectory,
        observed: { steps: ["context.compile", "capability:conversation.list:completed", "artifact:conversation_digest:v1"] }
      }, {
        id: "permission.shadow.e2d3583d196abdf176eaff43", category: "permission",
        expected: { decisions: [{ capabilityId: "conversation.list", resourceType: "conversation", resourceId: "user:u100", action: "read", decision: "allowed" }] },
        observed: { decisions: [{ capabilityId: "conversation.list", resourceType: "conversation", resourceId: "user:u100", action: "read", decision: "allowed" }] }
      }, {
        id: "retrieval.shadow.e2d3583d196abdf176eaff43", category: "retrieval",
        expected: manifest.labels.retrieval,
        observed: { retrievedEvidenceIds: ["evidence:9ca5a7ab8595d195421b6f96f544b8fb"] }
      }, {
        id: "cost.shadow.e2d3583d196abdf176eaff43", category: "cost",
        expected: { maximums: manifest.labels.cost.maximums },
        observed: { modelCalls: 1, toolCalls: 2, totalTokens: 150, totalCostMicrousd: 420, latencyMs: 770 }
      }]
    });
  });

  it("rejects incomplete or mismatched persisted evidence", () => {
    const manifest = parseShadowEvalManifest({
      schemaVersion: "dipole.agent.shadow-eval-manifest.v1", candidateVersion: "candidate/v1", taskId: "TASK-42", runId: "RUN-42",
      labels: {
        outcome: { requiredOutputIds: ["task:completed"], forbiddenOutputIds: [] },
        trajectory: { steps: [], forbiddenSteps: [] },
        permission: [{ stepNo: 1, capabilityId: "message.send", resourceType: "conversation", resourceId: "group:g1", action: "write", decision: "denied" }],
        retrieval: { relevantEvidenceIds: ["evidence:abc"], minimumRecall: 0, minimumPrecision: 0 },
        cost: {
          maximums: { modelCalls: 1, toolCalls: 1, totalTokens: 1, totalCostMicrousd: 1, latencyMs: 1 },
          routePrices: [{ route: "gateway/other", inputMicrousdPerMillionTokens: 1, outputMicrousdPerMillionTokens: 1 }]
        }
      }
    });
    const observed = observation();

    expect(() => buildShadowEvalSuite(manifest, { ...observed, taskId: "TASK-OTHER" })).toThrow(/Task binding/);
    expect(() => buildShadowEvalSuite(manifest, observed)).toThrow(/capability binding/);
  });

  it("uses a terminal durable Workflow when a Shadow policy Task remains running", () => {
    const observed = { ...observation(), taskStatus: "running", workflowStatus: "completed" };
    const manifest = parseShadowEvalManifest({
      schemaVersion: "dipole.agent.shadow-eval-manifest.v1", candidateVersion: "candidate/v1",
      taskId: observed.taskId, runId: observed.runId,
      labels: {
        outcome: { requiredOutputIds: ["task:completed", "run:completed"], forbiddenOutputIds: [] },
        trajectory: { steps: ["context.compile", "capability:conversation.list:completed", "artifact:conversation_digest:v1"], forbiddenSteps: [] },
        permission: [{ stepNo: 1, capabilityId: "conversation.list", resourceType: "conversation", resourceId: "user:u100", action: "read", decision: "allowed" }],
        retrieval: { relevantEvidenceIds: ["evidence:9ca5a7ab8595d195421b6f96f544b8fb"], minimumRecall: 1, minimumPrecision: 1 },
        cost: {
          maximums: { modelCalls: 1, toolCalls: 2, totalTokens: 300, totalCostMicrousd: 1000, latencyMs: 1000 },
          routePrices: [{ route: "gateway/primary", inputMicrousdPerMillionTokens: 2_000_000, outputMicrousdPerMillionTokens: 6_000_000 }]
        }
      }
    });

    expect(buildShadowEvalSuite(manifest, observed).cases[0]?.observed).toEqual({ outputIds: ["artifact:conversation_digest:v1", "run:completed", "task:completed"] });
  });

  it("rejects a running Shadow policy Task without a terminal durable Workflow", () => {
    const observed = { ...observation(), taskStatus: "running", workflowStatus: "running" };
    const manifest = parseShadowEvalManifest({
      schemaVersion: "dipole.agent.shadow-eval-manifest.v1", candidateVersion: "candidate/v1",
      taskId: observed.taskId, runId: observed.runId,
      labels: {
        outcome: { requiredOutputIds: ["task:completed"], forbiddenOutputIds: [] }, trajectory: { steps: [], forbiddenSteps: [] }, permission: [],
        retrieval: { relevantEvidenceIds: ["evidence:9ca5a7ab8595d195421b6f96f544b8fb"], minimumRecall: 0, minimumPrecision: 0 },
        cost: { maximums: { modelCalls: 1, toolCalls: 2, totalTokens: 300, totalCostMicrousd: 1000, latencyMs: 1000 }, routePrices: [{ route: "gateway/primary", inputMicrousdPerMillionTokens: 2_000_000, outputMicrousdPerMillionTokens: 6_000_000 }] }
      }
    });

    expect(() => buildShadowEvalSuite(manifest, observed)).toThrow(/Task execution and Run must be terminal/);
  });

  it("rejects a query sentinel row instead of silently truncating observations", () => {
    const base = observation();
    const manifest = parseShadowEvalManifest({
      schemaVersion: "dipole.agent.shadow-eval-manifest.v1", candidateVersion: "candidate/v1",
      taskId: base.taskId, runId: base.runId,
      labels: {
        outcome: { requiredOutputIds: ["task:completed"], forbiddenOutputIds: [] },
        trajectory: { steps: [], forbiddenSteps: [] }, permission: [],
        retrieval: { relevantEvidenceIds: ["evidence:abc"], minimumRecall: 0, minimumPrecision: 0 },
        cost: {
          maximums: { modelCalls: 64, toolCalls: 512, totalTokens: 1_000_000, totalCostMicrousd: 1_000_000, latencyMs: 1_000_000 },
          routePrices: [{ route: "gateway/primary", inputMicrousdPerMillionTokens: 1, outputMicrousdPerMillionTokens: 1 }]
        }
      }
    });

    expect(() => buildShadowEvalSuite(manifest, {
      ...base,
      artifacts: Array.from({ length: 257 }, (_, index) => ({ artifactType: "digest", version: index + 1 }))
    })).toThrow(/bounded collection limits/);
  });
});

function observation(): ShadowEvalObservation {
  return {
    taskId: "TASK-42", taskStatus: "completed", runId: "RUN-42", runStatus: "completed", traceId: "trace:adapter-42",
    contextManifest: {
      selected: [{ id: "event:E42", provenance: { sourceType: "kafka_event", sourceId: "E42" } }], omitted: []
    },
    steps: [{ stepNo: 1, capabilityId: "conversation.list", status: "completed", attemptCount: 1, latencyMs: 20 }],
    artifacts: [{ artifactType: "conversation_digest", version: 1 }],
    modelCalls: [{ route: "gateway/primary", status: "completed", inputTokens: 120, outputTokens: 30, latencyMs: 700 }],
    toolCalls: [{ status: "completed", latencyMs: 50 }]
  };
}
