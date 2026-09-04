import { describe, expect, it, vi } from "vitest";

import { evidenceId, type ShadowEvalObservation } from "./shadow-eval-adapter.js";
import { runShadowEvalReviewPackCLI } from "./shadow-eval-review-pack-cli.js";
import { buildShadowEvalReviewPack } from "./shadow-eval-review-pack.js";

describe("Shadow evaluation review pack", () => {
  it("exports only hashed bindings and resource scopes for a terminal observation", () => {
    const pack = buildShadowEvalReviewPack("agent-runtime@candidate-1", observation());
    const encoded = JSON.stringify(pack);

    expect(pack).toMatchObject({
      schemaVersion: "dipole.agent.shadow-eval-review-pack.v1",
      reviewStatus: "review_required",
      evaluatorEligibility: { status: "eligible", blockingReasons: [] },
      observed: {
        outputIds: ["artifact:conversation_digest:v1", "run:completed", "task:completed"],
        trajectory: ["context.compile", "capability:conversation.list:completed", "artifact:conversation_digest:v1"],
        permissions: [{ stepNo: 1, capabilityId: "conversation.list", status: "completed", authorization: { status: "complete", resourceType: "conversation", action: "read", decision: "allowed" } }],
        retrievedEvidenceIds: [evidenceId({ sourceType: "kafka_event", sourceId: "event:secret" })]
      }
    });
    expect(pack.binding.taskFingerprint).toMatch(/^sha256:[a-f0-9]{64}$/);
    const authorization = pack.observed.permissions[0]?.authorization;
    expect(authorization).toMatchObject({ status: "complete" });
    if (authorization?.status === "complete") expect(authorization.resourceFingerprint).toMatch(/^sha256:[a-f0-9]{64}$/);
    expect(encoded).not.toContain("TASK-SECRET");
    expect(encoded).not.toContain("RUN-SECRET");
    expect(encoded).not.toContain("trace:secret");
    expect(encoded).not.toContain("direct:U100:UAI");
    expect(encoded).not.toContain("event:secret");
  });

  it("rejects a non-terminal Task or Run, and classifies incomplete child evidence", () => {
    expect(() => buildShadowEvalReviewPack("agent-runtime@candidate-1", { ...observation(), runStatus: "running" }))
      .toThrow(/terminal Task execution and Run/);
    expect(buildShadowEvalReviewPack("agent-runtime@candidate-1", {
      ...observation(), modelCalls: [{ ...observation().modelCalls[0]!, status: "running" }]
    }).evaluatorEligibility).toEqual({ status: "blocked", blockingReasons: ["model_call_1_non_terminal"] });
    const pack = buildShadowEvalReviewPack("agent-runtime@candidate-1", {
      ...observation(), steps: [{ ...observation().steps[0]!, authorization: null }]
    });
    expect(pack.evaluatorEligibility).toEqual({ status: "blocked", blockingReasons: ["step_1_missing_authorization"] });
    expect(pack.observed.permissions[0]?.authorization).toEqual({ status: "missing" });
  });

  it("exports failed provider calls with unavailable token metering", () => {
    const pack = buildShadowEvalReviewPack("agent-runtime@candidate-1", {
      ...observation(),
      modelCalls: [{ route: "gateway/primary", status: "failed", inputTokens: null, outputTokens: null, latencyMs: 12 }]
    });

    expect(pack.evaluatorEligibility).toEqual({ status: "eligible", blockingReasons: [] });
    expect(pack.observed.metering.modelCalls).toEqual([
      { route: "gateway/primary", status: "failed", tokenMetering: "unavailable", latencyMetering: "complete" }
    ]);
  });

  it("keeps a trusted empty-discovery read visible without treating it as an unrecorded authorization", () => {
    const pack = buildShadowEvalReviewPack("agent-runtime@candidate-1", {
      ...observation(),
      steps: [
        observation().steps[0]!,
        { stepNo: 2, capabilityId: "conversation.read", status: "completed", attemptCount: 1, latencyMs: 0,
          authorization: null, skipReason: "no_discovered_conversation" }
      ]
    });

    expect(pack.evaluatorEligibility).toEqual({ status: "eligible", blockingReasons: [] });
    expect(pack.observed.trajectory).toContain("capability:conversation.read:skipped");
    expect(pack.observed.permissions[1]?.authorization).toEqual({ status: "not_required", reason: "no_discovered_conversation" });
  });

  it("loads one persisted observation through the read-only CLI", async () => {
    const output: string[] = [];
    const errors: string[] = [];
    const close = vi.fn(async () => undefined);
    const load = vi.fn(async () => observation());

    const code = await runShadowEvalReviewPackCLI([
      "--candidate-version=agent-runtime@candidate-1", "--task-id=TASK-SECRET", "--run-id=RUN-SECRET"
    ], writer(output), writer(errors), { openStore: () => ({ store: { load }, close }) });

    expect(code).toBe(0);
    expect(load).toHaveBeenCalledWith("TASK-SECRET", "RUN-SECRET");
    expect(close).toHaveBeenCalledOnce();
    expect(output.join("")).not.toContain("TASK-SECRET");
    expect(errors).toEqual([]);
  });

  it("rejects incomplete CLI arguments before opening the store", async () => {
    const errors: string[] = [];
    const openStore = vi.fn();
    await expect(runShadowEvalReviewPackCLI([], writer([]), writer(errors), { openStore })).resolves.toBe(1);
    expect(openStore).not.toHaveBeenCalled();
    expect(errors.join("")).toContain("requires exactly");
  });
});

function observation(): ShadowEvalObservation {
  return {
    taskId: "TASK-SECRET", taskStatus: "running", workflowStatus: "completed", runId: "RUN-SECRET", runStatus: "completed", traceId: "trace:secret",
    contextManifest: {
      selected: [
        { id: "event:secret", provenance: { sourceType: "kafka_event", sourceId: "event:secret" } },
        { id: "policy:secret", provenance: { sourceType: "runtime_policy", sourceId: "policy:secret" } }
      ],
      omitted: []
    },
    steps: [{
      stepNo: 1, capabilityId: "conversation.list", status: "completed", attemptCount: 1, latencyMs: 20,
      authorization: { resourceType: "conversation", resourceId: "direct:U100:UAI", action: "read", decision: "allowed" }
    }],
    artifacts: [{ artifactType: "conversation_digest", version: 1 }],
    modelCalls: [{ route: "gateway/primary", status: "completed", inputTokens: 120, outputTokens: 30, latencyMs: 700 }],
    toolCalls: [{ status: "completed", latencyMs: 50 }]
  };
}

function writer(values: string[]) {
  return { write: (value: string) => { values.push(String(value)); return true; } };
}
