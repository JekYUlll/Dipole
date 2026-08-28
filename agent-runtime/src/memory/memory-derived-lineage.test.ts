import { readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { describe, expect, it, vi } from "vitest";

import { runMemoryDerivedLineageCLI } from "./memory-derived-lineage-cli.js";
import { buildMemoryDerivedLineageReport, parseMemoryDerivedLineageManifest, parseMemoryDerivedLineageReport } from "./memory-derived-lineage.js";

describe("Agent Memory derived-lineage audit", () => {
  it("builds a low-sensitive conservative domain report", () => {
    const report = buildMemoryDerivedLineageReport(row());
    expect(report).toMatchObject({ lineageVersions: 2, directTaskReferences: 3, unindexedContextPlans: 1, unattributedModelTasks: 2, lineageComplete: false, contentRead: false, deletionAuthority: false, runtimeAuthority: false });
    expect(report.domains).toMatchObject({ modelCalls: 4, shadowPlans: 3, shadowSteps: 8, artifacts: 2, toolInvocations: 5, messageActions: 1, temporalHistoryPotentialTasks: 4 });
    expect(JSON.stringify(report)).not.toContain("MEM-ROOT-PRIVATE");
  });

  it("emits only the report and closes the read-only Store", async () => {
    const path = join(tmpdir(), `dipole-memory-lineage-${process.pid}-${Date.now()}.json`);
    await writeFile(path, JSON.stringify(manifest()), "utf8");
    const output: string[] = [], errors: string[] = [];
    const close = vi.fn(async () => undefined);
    const load = vi.fn(async () => buildMemoryDerivedLineageReport(row()));
    const code = await runMemoryDerivedLineageCLI([`--manifest=${path}`], writer(output), writer(errors), { openStore: () => ({ store: { load }, close }) });
    expect(code).toBe(0); expect(close).toHaveBeenCalledOnce(); expect(load).toHaveBeenCalledWith(manifest());
    expect(errors).toEqual([]); expect(output.join("")).not.toContain("U100"); expect(output.join("")).not.toContain("MEM-1");
  });

  it("fails closed without echoing manifest data", async () => {
    const path = join(tmpdir(), `dipole-memory-lineage-invalid-${process.pid}-${Date.now()}.json`);
    await writeFile(path, JSON.stringify({ ...manifest(), content: "private" }), "utf8");
    const errors: string[] = [];
    expect(await runMemoryDerivedLineageCLI([`--manifest=${path}`], writer([]), writer(errors))).toBe(1);
    expect(errors.join("")).toBe("Memory derived-lineage audit failed closed\n");
  });

  it("rejects reports with inconsistent lineage or authority", () => {
    const report = buildMemoryDerivedLineageReport(row());
    expect(() => parseMemoryDerivedLineageReport({ ...report, lineageComplete: true })).toThrow();
    expect(() => parseMemoryDerivedLineageReport({ ...report, deletionAuthority: true })).toThrow();
    expect(() => parseMemoryDerivedLineageReport({
      ...report,
      domains: { ...report.domains, temporalHistoryPotentialTasks: 2 }
    })).toThrow();
    expect(() => parseMemoryDerivedLineageReport({
      ...report,
      domains: { ...report.domains, modelCalls: 5 }
    })).toThrow(/hash/);
  });

  it("keeps the language-neutral examples aligned with runtime parsing", async () => {
    const contract = new URL("../../../contracts/agent-memory-derived-lineage/v1/", import.meta.url);
    const manifestExample = await readFile(new URL("manifest.example.json", contract), "utf8");
    const reportExample = await readFile(new URL("report.example.json", contract), "utf8");
    expect(parseMemoryDerivedLineageManifest(manifestExample)).toEqual(manifest());
    expect(parseMemoryDerivedLineageReport(reportExample)).toMatchObject({
      schemaVersion: "dipole.agent.memory-derived-lineage-report.v1",
      contentRead: false,
      deletionAuthority: false,
      runtimeAuthority: false
    });
  });
});

function manifest() { return { schemaVersion: "dipole.agent.memory-derived-lineage-manifest.v1", tenantId: "dipole", principalId: "U100", memoryId: "MEM-1" }; }
function row() { return { memory_root_uuid: "MEM-ROOT-PRIVATE", lineage_versions: 2, direct_task_references: 3, unindexed_context_plans: 1, unattributed_model_tasks: 2, model_calls: 4, shadow_plans: 3, shadow_steps: 8, artifacts: 2, tool_invocations: 5, message_actions: 1 }; }
function writer(values: string[]) { return { write: (value: string) => { values.push(String(value)); return true; } }; }
