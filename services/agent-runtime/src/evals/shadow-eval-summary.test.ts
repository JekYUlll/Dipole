import { writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { evaluateOfflineEvalSuite } from "./offline-evaluator.js";
import { createShadowEvalReport } from "./shadow-eval-report.js";
import { runShadowEvalSummaryCLI } from "./shadow-eval-summary-cli.js";
import { parseShadowEvalSummaryInput, summarizeShadowEvalReports } from "./shadow-eval-summary.js";

describe("Shadow evaluation summary", () => {
  it("reports reviewed sample task success and category-specific failures without source content", () => {
    const report = summarizeShadowEvalReports(parseShadowEvalSummaryInput(input([passingReport(), failingReport()])));

    expect(report).toMatchObject({
      schemaVersion: "dipole.agent.shadow-eval-summary-report.v1",
      candidateVersion: "candidate/v1",
      summary: {
        evaluatedTasks: 2, succeededTasks: 1, failedTasks: 1, taskSuccessRatePercent: 50,
        categoryPassRates: { outcome: { total: 2, passed: 1, passRatePercent: 50 } }
      }
    });
    expect(report.summary.failureReasons).toContainEqual({ category: "outcome", reason: "missing_required_output", count: 1 });
    expect(report.traceIds).toEqual(["trace:one", "trace:two"]);
    expect(JSON.stringify(report)).not.toContain("TASK-SECRET");
    expect(JSON.stringify(report)).not.toContain("prompt body");
  });

  it("rejects a mixed candidate window and duplicate evaluation evidence", () => {
    const candidateDrift = passingReport();
    expect(() => parseShadowEvalSummaryInput(input([passingReport(), { ...candidateDrift, evaluation: { ...candidateDrift.evaluation, candidateVersion: "candidate/v2" } }]))).toThrow("candidate version");
    expect(() => parseShadowEvalSummaryInput(input([passingReport(), passingReport()]))).toThrow("unique suite SHA-256");
    const shadowReport = passingReport();
    const malformedReport = {
      ...shadowReport,
      evaluation: {
        ...shadowReport.evaluation,
        cases: [{ ...shadowReport.evaluation.cases[0]!, id: "outcome.offline.fixture" }, ...shadowReport.evaluation.cases.slice(1)]
      }
    };
    expect(() => parseShadowEvalSummaryInput(input([malformedReport]))).toThrow("bound Shadow case");
  });

  it("writes a valid report and uses exit code two for reviewed failures", async () => {
    const path = join(tmpdir(), `dipole-shadow-summary-${process.pid}-${Date.now()}.json`);
    await writeFile(path, JSON.stringify(input([failingReport()])), "utf8");
    const stdout: string[] = [];
    const stderr: string[] = [];

    await expect(runShadowEvalSummaryCLI([`--input=${path}`], writer(stdout), writer(stderr))).resolves.toBe(2);
    expect(JSON.parse(stdout.join(""))).toMatchObject({ summary: { taskSuccessRatePercent: 0 } });
    expect(stderr).toEqual([]);
  });

  it("accepts a 40-character Git revision from an OCI runtime image", () => {
    expect(parseShadowEvalSummaryInput(input([passingReport()], "a".repeat(40))).source.runtimeRevision).toHaveLength(40);
  });

  it("fails closed for invalid arguments", async () => {
    const stderr: string[] = [];
    await expect(runShadowEvalSummaryCLI([], writer([]), writer(stderr))).resolves.toBe(1);
    expect(stderr.join("")).toContain("exactly one --input");
  });
});

function input(reports: readonly ReturnType<typeof passingReport>[], runtimeRevision = "a".repeat(64)) {
  return {
    schemaVersion: "dipole.agent.shadow-eval-summary-input.v1",
    source: {
      kind: "reviewed_shadow", environment: "isolated",
      runtimeRevision, windowStart: "2026-08-31T00:00:00.000Z", windowEnd: "2026-08-31T01:00:00.000Z"
    },
    reports
  };
}

function passingReport() {
  return createShadowEvalReport("trace:one", evaluateOfflineEvalSuite({
    schemaVersion: "dipole.agent.offline-eval-suite.v1", candidateVersion: "candidate/v1",
    cases: [
      { id: `outcome.shadow.${"a".repeat(24)}`, category: "outcome", expected: { requiredOutputIds: ["task:completed"], forbiddenOutputIds: [] }, observed: { outputIds: ["task:completed"] } },
      { id: `trajectory.shadow.${"a".repeat(24)}`, category: "trajectory", expected: { steps: [], forbiddenSteps: [] }, observed: { steps: [] } },
      { id: `permission.shadow.${"a".repeat(24)}`, category: "permission", expected: { decisions: [] }, observed: { decisions: [] } },
      { id: `retrieval.shadow.${"a".repeat(24)}`, category: "retrieval", expected: { relevantEvidenceIds: ["evidence:one"], minimumRecall: 1, minimumPrecision: 1 }, observed: { retrievedEvidenceIds: ["evidence:one"] } },
      { id: `cost.shadow.${"a".repeat(24)}`, category: "cost", expected: { maximums: { modelCalls: 1, toolCalls: 1, totalTokens: 10, totalCostMicrousd: 10, latencyMs: 10 } }, observed: { modelCalls: 1, toolCalls: 1, totalTokens: 10, totalCostMicrousd: 10, latencyMs: 10, tokenMetrics: "complete" } }
    ]
  }));
}

function failingReport() {
  return createShadowEvalReport("trace:two", evaluateOfflineEvalSuite({
    schemaVersion: "dipole.agent.offline-eval-suite.v1", candidateVersion: "candidate/v1",
    cases: [
      { id: `outcome.shadow.${"b".repeat(24)}`, category: "outcome", expected: { requiredOutputIds: ["task:completed"], forbiddenOutputIds: [] }, observed: { outputIds: [] } },
      { id: `trajectory.shadow.${"b".repeat(24)}`, category: "trajectory", expected: { steps: [], forbiddenSteps: [] }, observed: { steps: [] } },
      { id: `permission.shadow.${"b".repeat(24)}`, category: "permission", expected: { decisions: [] }, observed: { decisions: [] } },
      { id: `retrieval.shadow.${"b".repeat(24)}`, category: "retrieval", expected: { relevantEvidenceIds: ["evidence:two"], minimumRecall: 1, minimumPrecision: 1 }, observed: { retrievedEvidenceIds: ["evidence:two"] } },
      { id: `cost.shadow.${"b".repeat(24)}`, category: "cost", expected: { maximums: { modelCalls: 1, toolCalls: 1, totalTokens: 10, totalCostMicrousd: 10, latencyMs: 10 } }, observed: { modelCalls: 1, toolCalls: 1, totalTokens: 10, totalCostMicrousd: 10, latencyMs: 10, tokenMetrics: "complete" } }
    ]
  }));
}

function writer(values: string[]) {
  return { write: (value: string) => { values.push(String(value)); return true; } };
}
