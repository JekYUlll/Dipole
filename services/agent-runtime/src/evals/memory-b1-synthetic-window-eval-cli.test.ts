import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { runMemoryB1SyntheticWindowCLI } from "./memory-b1-synthetic-window-eval-cli.js";

describe("B1 synthetic Memory window CLI", () => {
  it("writes a valid failed window report", async () => {
    const directory = await mkdtemp(join(tmpdir(), "dipole-b1-window-"));
    const windowPath = join(directory, "window.json");
    const reportPath = join(directory, "report.json");
    await writeFile(windowPath, JSON.stringify(window()));
    const code = await runMemoryB1SyntheticWindowCLI([`--window=${windowPath}`, `--report=${reportPath}`], { write: () => undefined }, { write: () => undefined });
    expect(code).toBe(2);
    expect(JSON.parse(await readFile(reportPath, "utf8"))).toMatchObject({ passed: false, metrics: { recallPassBps: 6666 } });
  });
});

function window() {
  const hash = (value: string) => value.repeat(64).slice(0, 64);
  const suite = (canary: string, recalled: boolean) => ({ schemaVersion: "dipole.agent.memory-b1-synthetic-eval.v1", candidateVersion: "agent-runtime@window-cli", minimumPassBps: 10_000, cases: [{ caseId: "recall", canarySha256: hash(canary), expectedMemoryLineageCount: 1, recallExpectation: "required" }, { caseId: "revoke", canarySha256: hash(canary), expectedMemoryLineageCount: 0, recallExpectation: "not_evaluated" }], observations: [{ caseId: "recall", canarySha256: hash(canary), taskSha256: hash(`${canary}a`), replySha256: hash(`${canary}b`), taskCompleted: true, modelCallCount: 1, memoryLineageCount: 1, responseContainsCanary: recalled }, { caseId: "revoke", canarySha256: hash(canary), taskSha256: hash(`${canary}c`), replySha256: hash(`${canary}d`), taskCompleted: true, modelCallCount: 1, memoryLineageCount: 0, responseContainsCanary: false }] });
  return { schemaVersion: "dipole.agent.memory-b1-synthetic-window.v1", candidateVersion: "agent-runtime@window-cli", minimumRecallPassBps: 10_000, suites: [suite("a", true), suite("b", true), suite("c", false)] };
}
