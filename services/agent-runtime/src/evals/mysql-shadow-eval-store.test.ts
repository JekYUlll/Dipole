import { describe, expect, it, vi } from "vitest";

import { MySQLShadowEvalObservationStore } from "./mysql-shadow-eval-store.js";

describe("MySQL Shadow evaluation observation store", () => {
  it("loads a bounded low-sensitivity snapshot in deterministic order", async () => {
    const execute = vi.fn()
      .mockResolvedValueOnce([[{
        task_uuid: "TASK-1", task_status: "completed", run_uuid: "RUN-1", run_status: "completed",
        context_manifest_json: JSON.stringify({ selected: [], omitted: [] })
      }]])
      .mockResolvedValueOnce([[{ step_no: 1, capability_id: "conversation.list", status: "completed", attempt_count: 1, latency_ms: 2 }]])
      .mockResolvedValueOnce([[{ artifact_type: "digest", version: 1 }]])
      .mockResolvedValueOnce([[{
        route: "gateway/primary", status: "completed", input_tokens: 12, output_tokens: 3, latency_ms: 40
      }]])
      .mockResolvedValueOnce([[{ status: "completed", latency_ms: 5 }]]);
    const store = new MySQLShadowEvalObservationStore({ execute } as never);

    await expect(store.load("TASK-1", "RUN-1")).resolves.toEqual({
      taskId: "TASK-1", taskStatus: "completed", runId: "RUN-1", runStatus: "completed",
      contextManifest: { selected: [], omitted: [] },
      steps: [{ stepNo: 1, capabilityId: "conversation.list", status: "completed", attemptCount: 1, latencyMs: 2 }],
      artifacts: [{ artifactType: "digest", version: 1 }],
      modelCalls: [{ route: "gateway/primary", status: "completed", inputTokens: 12, outputTokens: 3, latencyMs: 40 }],
      toolCalls: [{ status: "completed", latencyMs: 5 }]
    });
    expect(execute).toHaveBeenCalledTimes(5);
  });

  it("fails closed when the requested Task and Run are absent", async () => {
    const store = new MySQLShadowEvalObservationStore({ execute: vi.fn().mockResolvedValue([[]]) } as never);
    await expect(store.load("TASK-404", "RUN-404")).rejects.toThrow(/missing/);
  });
});
