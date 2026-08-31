import { writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it, vi } from "vitest";

import type { ShadowEvalObservation } from "./shadow-eval-adapter.js";
import { runShadowEvalCLI } from "./shadow-eval-cli.js";

describe("Shadow evaluation CLI", () => {
  it("loads a bound Task observation and emits only the low-sensitivity report", async () => {
    const path = join(tmpdir(), `dipole-shadow-eval-${process.pid}-${Date.now()}.json`);
    await writeFile(path, JSON.stringify(manifest()), "utf8");
    const output: string[] = [];
    const errors: string[] = [];
    const close = vi.fn(async () => undefined);
    const load = vi.fn(async () => observation());

    const code = await runShadowEvalCLI(
      [`--manifest=${path}`], writer(output), writer(errors), { openStore: () => ({ store: { load }, close }) }
    );

    expect(code).toBe(0);
    expect(load).toHaveBeenCalledWith("TASK-1", "RUN-1");
    expect(close).toHaveBeenCalledOnce();
    expect(JSON.parse(output.join(""))).toMatchObject({ evaluation: { candidateVersion: "candidate/v1", passed: true }, traceId: "trace:shadow-1" });
    expect(output.join("")).not.toContain("E1");
    expect(errors).toEqual([]);
  });

  it("uses invalid-input exit code for missing arguments", async () => {
    const errors: string[] = [];
    await expect(runShadowEvalCLI([], process.stdout, writer(errors))).resolves.toBe(1);
    expect(errors.join("")).toContain("exactly one --manifest");
  });
});

function manifest() {
  return {
    schemaVersion: "dipole.agent.shadow-eval-manifest.v1", candidateVersion: "candidate/v1", taskId: "TASK-1", runId: "RUN-1",
    labels: {
      outcome: { requiredOutputIds: ["task:completed"], forbiddenOutputIds: [] },
      trajectory: { steps: ["context.compile", "capability:conversation.list:completed"], forbiddenSteps: [] },
      permission: [{ stepNo: 1, capabilityId: "conversation.list", resourceType: "conversation", resourceId: "user:u1", action: "read", decision: "allowed" }],
      retrieval: { relevantEvidenceIds: ["evidence:d83b80e38108fe93fc51f85510e4a7d6"], minimumRecall: 1, minimumPrecision: 1 },
      cost: {
        maximums: { modelCalls: 1, toolCalls: 1, totalTokens: 20, totalCostMicrousd: 20, latencyMs: 20 },
        routePrices: [{ route: "gateway/primary", inputMicrousdPerMillionTokens: 1_000_000, outputMicrousdPerMillionTokens: 1_000_000 }]
      }
    }
  };
}

function observation(): ShadowEvalObservation {
  return {
    taskId: "TASK-1", taskStatus: "completed", runId: "RUN-1", runStatus: "completed", traceId: "trace:shadow-1",
    contextManifest: { selected: [{ id: "event:E1", provenance: { sourceType: "kafka_event", sourceId: "E1" } }], omitted: [] },
    steps: [{ stepNo: 1, capabilityId: "conversation.list", status: "completed", attemptCount: 1, latencyMs: 3 }], artifacts: [],
    modelCalls: [{ route: "gateway/primary", status: "completed", inputTokens: 10, outputTokens: 2, latencyMs: 10 }], toolCalls: []
  };
}

function writer(values: string[]) {
  return { write: (value: string) => { values.push(String(value)); return true; } };
}
