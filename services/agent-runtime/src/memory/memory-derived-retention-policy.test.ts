import { readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { runMemoryDerivedRetentionPolicyCLI } from "./memory-derived-retention-policy-cli.js";
import {
  buildMemoryDerivedRetentionDecision,
  parseMemoryDerivedRetentionDecision,
  parseMemoryDerivedRetentionPolicy,
  type MemoryDerivedRetentionPolicy
} from "./memory-derived-retention-policy.js";
import { buildMemoryDerivedLineageReport } from "./memory-derived-lineage.js";

describe("Agent Memory derived retention policy", () => {
  it("builds a deterministic low-sensitive decision for complete lineage", () => {
    const report = buildMemoryDerivedLineageReport(row());
    const decision = buildMemoryDerivedRetentionDecision(policy(), report);

    expect(decision).toMatchObject({
      memoryRootSha256: report.memoryRootSha256,
      lineageReportSha256: report.reportSha256,
      lineageComplete: true,
      policyComplete: true,
      blockedReasons: [],
      contentRead: false,
      deletionExecuted: false,
      deletionAuthority: false,
      runtimeAuthority: false
    });
    expect(decision.domains.shadowPlans).toEqual({ count: 1, action: "erase_derived_content" });
    expect(JSON.stringify(decision)).not.toContain("MEM-ROOT-PRIVATE");
    expect(buildMemoryDerivedRetentionDecision(policy(), report)).toEqual(decision);
  });

  it("fails closed when lineage is incomplete", () => {
    const report = buildMemoryDerivedLineageReport({ ...row(), unindexed_context_plans: 1 });
    const decision = buildMemoryDerivedRetentionDecision(policy(), report);

    expect(decision.policyComplete).toBe(false);
    expect(decision.blockedReasons).toEqual(["lineage_incomplete"]);
  });

  it("requires review only when the reviewed domain has impact", () => {
    const report = buildMemoryDerivedLineageReport(row());
    const reviewed = policy({ artifacts: { action: "manual_review" } });
    expect(buildMemoryDerivedRetentionDecision(reviewed, report).blockedReasons).toEqual(["manual_review_required"]);

    const noArtifactImpact = buildMemoryDerivedLineageReport({ ...row(), artifacts: 0 });
    expect(buildMemoryDerivedRetentionDecision(reviewed, noArtifactImpact).policyComplete).toBe(true);
  });

  it("rejects policy, report and decision drift", () => {
    expect(() => parseMemoryDerivedRetentionPolicy({ ...policy(), deletionAuthority: true })).toThrow();
    const report = buildMemoryDerivedLineageReport(row());
    expect(() => buildMemoryDerivedRetentionDecision(policy(), { ...report, reportSha256: "0".repeat(64) })).toThrow(/hash/);
    const decision = buildMemoryDerivedRetentionDecision(policy(), report);
    expect(() => parseMemoryDerivedRetentionDecision({ ...decision, policyComplete: false })).toThrow();
    expect(() => parseMemoryDerivedRetentionDecision({
      ...decision,
      domains: { ...decision.domains, artifacts: { count: 1, action: "manual_review" } }
    })).toThrow();
    expect(() => parseMemoryDerivedRetentionDecision({ ...decision, deletionExecuted: true })).toThrow();
  });

  it("runs as a bounded offline CLI and keeps examples aligned", async () => {
    const contract = new URL("../../../../contracts/agent-memory-derived-retention/v1/", import.meta.url);
    const policyText = await readFile(new URL("policy.example.json", contract), "utf8");
    const reportText = await readFile(new URL("report.example.json", contract), "utf8");
    const expectedText = await readFile(new URL("decision.example.json", contract), "utf8");
    const parsedPolicy = parseMemoryDerivedRetentionPolicy(policyText);
    const expected = parseMemoryDerivedRetentionDecision(expectedText);

    const suffix = `${process.pid}-${Date.now()}`;
    const policyPath = join(tmpdir(), `dipole-derived-retention-policy-${suffix}.json`);
    const reportPath = join(tmpdir(), `dipole-derived-retention-report-${suffix}.json`);
    await writeFile(policyPath, policyText, "utf8");
    await writeFile(reportPath, reportText, "utf8");
    const output: string[] = [], errors: string[] = [];
    const code = await runMemoryDerivedRetentionPolicyCLI(
      [`--policy=${policyPath}`, `--report=${reportPath}`], writer(output), writer(errors)
    );

    expect(code).toBe(0);
    expect(errors).toEqual([]);
    expect(parseMemoryDerivedRetentionDecision(output.join(""))).toEqual(expected);
    expect(parsedPolicy.policyVersion).toBe("memory-derived-retention-v1");
  });

  it("fails closed without echoing paths or input", async () => {
    const path = join(tmpdir(), `dipole-derived-retention-private-${process.pid}-${Date.now()}.json`);
    await writeFile(path, JSON.stringify({ secret: "private" }), "utf8");
    const output: string[] = [], errors: string[] = [];
    expect(await runMemoryDerivedRetentionPolicyCLI(
      [`--policy=${path}`, `--report=${path}`], writer(output), writer(errors)
    )).toBe(1);
    expect(output).toEqual([]);
    expect(errors.join("")).toBe("Memory derived retention decision failed closed\n");

    const oversized = join(tmpdir(), `dipole-derived-retention-oversized-${process.pid}-${Date.now()}.json`);
    await writeFile(oversized, "x".repeat(64 * 1024 + 1), "utf8");
    expect(await runMemoryDerivedRetentionPolicyCLI(
      [`--policy=${oversized}`, `--report=${path}`], writer(output), writer(errors)
    )).toBe(1);
    expect(errors.at(-1)).toBe("Memory derived retention decision failed closed\n");
  });
});

function policy(overrides: Partial<MemoryDerivedRetentionPolicy["domains"]> = {}): MemoryDerivedRetentionPolicy {
  return {
    schemaVersion: "dipole.agent.memory-derived-retention-policy.v1",
    policyVersion: "memory-derived-retention-v1",
    domains: {
      modelCalls: { action: "retain_minimal_audit" },
      shadowPlans: { action: "erase_derived_content" },
      shadowSteps: { action: "erase_derived_content" },
      artifacts: { action: "erase_derived_content" },
      toolInvocations: { action: "retain_minimal_audit" },
      messageActions: { action: "retain_minimal_audit" },
      temporalHistoryPotentialTasks: { action: "expire_after_days", retentionDays: 30 },
      ...overrides
    },
    contentRead: false,
    deletionAuthority: false,
    runtimeAuthority: false
  };
}

function row() {
  return {
    memory_root_uuid: "MEM-ROOT-PRIVATE", lineage_versions: 2, direct_task_references: 1,
    unindexed_context_plans: 0, unattributed_model_tasks: 0, model_calls: 1, shadow_plans: 1,
    shadow_steps: 2, artifacts: 1, tool_invocations: 1, message_actions: 1
  };
}

function writer(values: string[]) { return { write: (value: string) => { values.push(String(value)); return true; } }; }
