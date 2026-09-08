import { mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { runMemoryB1SyntheticEvalCLI } from "./memory-b1-synthetic-eval-cli.js";

describe("B1 synthetic Memory Eval CLI", () => {
  it("emits a low-sensitive report for a valid suite", async () => {
    const path = join(await mkdtemp(join(tmpdir(), "dipole-b1-eval-")), "suite.json");
    await writeFile(path, JSON.stringify(fixture()));
    const output: string[] = [];
    const code = await runMemoryB1SyntheticEvalCLI([`--suite=${path}`], { write: value => output.push(String(value)) }, { write: () => undefined });
    expect(code).toBe(0);
    expect(JSON.parse(output.join(""))).toMatchObject({ schemaVersion: "dipole.agent.memory-b1-synthetic-eval-report.v1", passed: true, metrics: { totalCases: 2 } });
  });

  it("rejects invalid arguments", async () => {
    const errors: string[] = [];
    await expect(runMemoryB1SyntheticEvalCLI([], { write: () => undefined }, { write: value => errors.push(String(value)) })).resolves.toBe(1);
    expect(errors.join("")).toContain("requires exactly one");
  });
});

function fixture() {
  const hash = (value: string) => value.repeat(64).slice(0, 64);
  return {
    schemaVersion: "dipole.agent.memory-b1-synthetic-eval.v1", candidateVersion: "agent-runtime@fixture", minimumPassBps: 10_000,
    cases: [{ caseId: "recall", canarySha256: hash("a"), expectedMemoryLineageCount: 1, recallExpectation: "required" }, { caseId: "revoke", canarySha256: hash("a"), expectedMemoryLineageCount: 0, recallExpectation: "not_evaluated" }],
    observations: [{ caseId: "recall", canarySha256: hash("a"), taskSha256: hash("b"), replySha256: hash("c"), taskCompleted: true, modelCallCount: 1, memoryLineageCount: 1, responseContainsCanary: true }, { caseId: "revoke", canarySha256: hash("a"), taskSha256: hash("d"), replySha256: hash("e"), taskCompleted: true, modelCallCount: 1, memoryLineageCount: 0, responseContainsCanary: true }]
  };
}
