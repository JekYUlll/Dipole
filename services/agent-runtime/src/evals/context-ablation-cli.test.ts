import { writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { describe, expect, it, vi } from "vitest";

import { runContextAblationCLI } from "./context-ablation-cli.js";
import type { ContextAblationCaseObservation } from "./mysql-context-ablation-store.js";

const sha = "a".repeat(64);

describe("Context Ablation CLI", () => {
  it("loads a reviewed experiment and emits only the aggregate report", async () => {
    const path = join(tmpdir(), `dipole-context-ablation-${process.pid}-${Date.now()}.json`);
    await writeFile(path, JSON.stringify(manifest()), "utf8");
    const output: string[] = [];
    const errors: string[] = [];
    const close = vi.fn(async () => undefined);
    const load = vi.fn(async () => [observation()]);

    const code = await runContextAblationCLI([`--manifest=${path}`], writer(output), writer(errors), { openStore: () => ({ store: { load }, close }) });

    expect(code).toBe(0);
    expect(load).toHaveBeenCalledWith("experiment:1");
    expect(close).toHaveBeenCalledOnce();
    expect(JSON.parse(output.join(""))).toMatchObject({ schemaVersion: "dipole.agent.context-ablation-report.v1", candidateVersion: "agent@1", caseCount: 1 });
    expect(output.join("")).not.toContain("source:1");
    expect(errors).toEqual([]);
  });

  it("returns invalid-input exit code without opening a database handle", async () => {
    const errors: string[] = [];
    const openStore = vi.fn();
    await expect(runContextAblationCLI([], process.stdout, writer(errors), { openStore })).resolves.toBe(1);
    expect(openStore).not.toHaveBeenCalled();
    expect(errors.join("")).toContain("exactly one --manifest");
  });
});

function manifest() {
  return {
    schemaVersion: "dipole.agent.context-ablation-manifest.v1", experimentId: "experiment:1", candidateVersion: "agent@1",
    routePrices: [{ route: "deepseek/flash", inputMicrousdPerMillionTokens: 1_000_000, outputMicrousdPerMillionTokens: 2_000_000 }],
    cases: [{ caseSha256: sha, requiredOutputIds: ["artifact:conversation_digest:v1"], relevantEvidenceIds: ["evidence:741f3775ecac87427a5963b4d12ea336"] }]
  };
}

function observation(): ContextAblationCaseObservation {
  const item = {
    taskId: "task:1", taskStatus: "completed", runId: "run:1", runStatus: "completed", traceId: "trace:1",
    contextManifest: { selected: [{ id: "selected:1", provenance: { sourceType: "conversation", sourceId: "source:1" } }], omitted: [] },
    steps: [{ stepNo: 1, capabilityId: "conversation.read", status: "completed", attemptCount: 1, latencyMs: 3, authorization: { resourceType: "conversation", resourceId: "conversation:1", action: "read", decision: "allowed" as const } }],
    artifacts: [{ artifactType: "conversation_digest", version: 1 }],
    modelCalls: [{ route: "deepseek/flash", status: "completed", inputTokens: 2, outputTokens: 3, latencyMs: 4 }], toolCalls: [{ status: "completed", latencyMs: 5 }]
  };
  return { caseSha256: sha, candidateVersion: "agent@1", observations: { baseline: item, retrieval: item, memory: item } };
}

function writer(values: string[]) { return { write: (value: string) => { values.push(String(value)); return true; } }; }
