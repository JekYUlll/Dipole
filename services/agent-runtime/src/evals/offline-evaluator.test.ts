import { readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { evaluateOfflineEvalSuite, parseOfflineEvalReport, parseOfflineEvalSuite } from "./offline-evaluator.js";
import { runOfflineEvalCLI } from "./offline-eval-cli.js";

describe("Agent offline evaluator", () => {
  it("evaluates all five deterministic categories and binds a stable suite hash", async () => {
    const suite = parseOfflineEvalSuite(await fixture());
    const first = evaluateOfflineEvalSuite(suite);
    const second = evaluateOfflineEvalSuite(structuredClone(suite));

    expect(first).toEqual(second);
    expect(first).toMatchObject({
      schemaVersion: "dipole.agent.offline-eval-report.v1",
      candidateVersion: "agent-runtime@candidate-1",
      passed: true,
      summary: { total: 5, passed: 5 }
    });
    expect(first.suiteSha256).toMatch(/^[a-f0-9]{64}$/);
    expect(first.summary.categories).toEqual({
      outcome: { total: 1, passed: 1 }, trajectory: { total: 1, passed: 1 }, permission: { total: 1, passed: 1 },
      retrieval: { total: 1, passed: 1 }, cost: { total: 1, passed: 1 }
    });
  });

  it("reports bounded category-specific failures without echoing content", async () => {
    const suite = parseOfflineEvalSuite(await fixture());
    const cases = suite.cases;
    const outcome = cases.find(item => item.category === "outcome");
    const trajectory = cases.find(item => item.category === "trajectory");
    const permission = cases.find(item => item.category === "permission");
    const retrieval = cases.find(item => item.category === "retrieval");
    const cost = cases.find(item => item.category === "cost");
    if (outcome === undefined || trajectory === undefined || permission === undefined || retrieval === undefined || cost === undefined) {
      throw new Error("fixture must contain all five categories");
    }
    outcome.observed.outputIds = ["message:unexpected"];
    trajectory.observed.steps = ["context.compile", "tool.message.send"];
    permission.observed.decisions[0]!.decision = "allowed";
    retrieval.observed.retrievedEvidenceIds = ["evidence:irrelevant"];
    cost.observed.totalTokens = 5000;

    const report = evaluateOfflineEvalSuite(suite);

    expect(report.passed).toBe(false);
    expect(report.cases.map((item) => item.reasons)).toEqual([
      ["forbidden_output", "missing_required_output"],
      ["forbidden_step", "trajectory_mismatch"],
      ["permission_decision_mismatch"],
      ["retrieval_precision_below_minimum", "retrieval_recall_below_minimum"],
      ["total_tokens_exceeded"]
    ]);
    expect(JSON.stringify(report)).not.toContain("sensitive conversation body");
  });

  it("fails cost evaluation when token metering is unavailable without treating it as zero", async () => {
    const suite = JSON.parse(await fixture()) as any;
    suite.cases[4].observed = {
      modelCalls: 1, toolCalls: 0, totalTokens: 0, totalCostMicrousd: 0, latencyMs: 91,
      tokenMetrics: "unavailable"
    };

    const report = evaluateOfflineEvalSuite(parseOfflineEvalSuite(suite));

    expect(report.cases[4]).toMatchObject({
      passed: false,
      reasons: ["token_metrics_unavailable"],
      metrics: { modelCalls: 1, toolCalls: 0, totalTokens: 0, totalCostMicrousd: 0, latencyMs: 91 },
      availability: { tokenMetrics: "unavailable" }
    });
  });

  it("rejects duplicate cases, missing categories, unknown fields, and invalid thresholds", async () => {
    const duplicate = JSON.parse(await fixture()) as any;
    duplicate.cases[1].id = duplicate.cases[0].id;
    expect(() => parseOfflineEvalSuite(duplicate)).toThrow(/unique/);

    const missing = JSON.parse(await fixture()) as any;
    missing.cases.pop();
    expect(() => parseOfflineEvalSuite(missing)).toThrow(/all five/);

    const unknown = JSON.parse(await fixture()) as any;
    unknown.cases[0].unexpected = true;
    expect(() => parseOfflineEvalSuite(unknown)).toThrow();

    const threshold = JSON.parse(await fixture()) as any;
    threshold.cases[3].expected.minimumRecall = 1.1;
    expect(() => parseOfflineEvalSuite(threshold)).toThrow();
  });

  it("preserves repeated trajectory steps while keeping set-like identifiers unique", async () => {
    const suite = JSON.parse(await fixture()) as any;
    suite.cases[1].expected.steps = ["tool.conversation.list", "tool.conversation.list"];
    suite.cases[1].observed.steps = ["tool.conversation.list", "tool.conversation.list"];

    expect(evaluateOfflineEvalSuite(parseOfflineEvalSuite(suite)).cases[1]).toMatchObject({ passed: true });

    suite.cases[0].observed.outputIds = ["artifact:conversation_digest", "artifact:conversation_digest"];
    expect(() => parseOfflineEvalSuite(suite)).toThrow(/unique/);
  });

  it("rejects reports whose case decision conflicts with its reasons", async () => {
    const report = evaluateOfflineEvalSuite(parseOfflineEvalSuite(await fixture()));
    report.cases[0]!.reasons = ["missing_required_output"];

    expect(() => parseOfflineEvalReport(report)).toThrow(/case result/);
  });

  it("provides a reproducible CLI with pass, fail, and invalid exit codes", async () => {
    const path = join(tmpdir(), `dipole-agent-eval-${process.pid}-${Date.now()}.json`);
    const output: string[] = [];
    const errors: string[] = [];
    await writeFile(path, await fixture(), "utf8");
    await expect(runOfflineEvalCLI([`--suite=${path}`], { write: (value) => { output.push(String(value)); return true; } }, { write: (value) => { errors.push(String(value)); return true; } })).resolves.toBe(0);
    expect(JSON.parse(output.join(""))).toMatchObject({ passed: true, summary: { total: 5 } });

    const failed = JSON.parse(await fixture()) as any;
    failed.cases[4].observed.modelCalls = 2;
    await writeFile(path, JSON.stringify(failed), "utf8");
    output.length = 0;
    await expect(runOfflineEvalCLI([`--suite=${path}`], { write: (value) => { output.push(String(value)); return true; } }, process.stderr)).resolves.toBe(2);
    expect(JSON.parse(output.join(""))).toMatchObject({ passed: false });

    await expect(runOfflineEvalCLI([], process.stdout, { write: (value) => { errors.push(String(value)); return true; } })).resolves.toBe(1);
    expect(errors.join(" ")).toContain("exactly one --suite");
  });
});

async function fixture(): Promise<string> {
  return readFile(new URL("../../../../contracts/agent-evals/v1/offline-suite.json", import.meta.url), "utf8");
}
