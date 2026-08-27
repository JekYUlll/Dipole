import { readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { evaluateOfflineEvalSuite, parseOfflineEvalSuite } from "../evals/offline-evaluator.js";
import { runPromotionCheckCLI } from "./promotion-check-cli.js";

describe("Agent promotion check CLI", () => {
  it("dispatches hash-bound promotion v2 evidence and rejects invalid input", async () => {
    const suitePath = new URL("../../../contracts/agent-evals/v1/offline-suite.json", import.meta.url);
    const suite = parseOfflineEvalSuite(await readFile(suitePath, "utf8"));
    suite.candidateVersion = "agent-runtime@abc1234";
    const path = join(tmpdir(), `dipole-agent-promotion-${process.pid}-${Date.now()}.json`);
    await writeFile(path, JSON.stringify(evidence(evaluateOfflineEvalSuite(suite))), "utf8");
    const output: string[] = [];
    const errors: string[] = [];

    await expect(runPromotionCheckCLI([`--evidence=${path}`], sink(output), sink(errors))).resolves.toBe(0);
    expect(JSON.parse(output.join(""))).toMatchObject({
      schemaVersion: "dipole.agent.shadow-promotion-decision.v2",
      decision: "eligible"
    });
    await expect(runPromotionCheckCLI([], sink(output), sink(errors))).resolves.toBe(1);
    expect(errors.join(" ")).toContain("exactly one --evidence");
  });
});

function evidence(offlineEvalReport: ReturnType<typeof evaluateOfflineEvalSuite>): object {
  const started = Date.parse("2026-08-27T00:00:00.000Z");
  return {
    schemaVersion: "dipole.agent.shadow-promotion-evidence.v2",
    candidateVersion: "agent-runtime@abc1234",
    windowStartedAt: new Date(started).toISOString(),
    windowEndedAt: new Date(started + 24 * 60 * 60 * 1000).toISOString(),
    observations: Array.from({ length: 25 }, (_, index) => ({
      candidateVersion: "agent-runtime@abc1234",
      observedAt: new Date(started + index * 60 * 60 * 1000).toISOString(),
      report: {
        schemaVersion: "dipole.agent.projection-reconcile.v1",
        consistent: true,
        scanned: 5,
        outcomes: { match: 5, missing: 0, stale: 0, ahead: 0, conflict: 0, unavailable: 0 },
        examples: []
      }
    })),
    projectionEvals: { passed: 6, total: 6 },
    offlineEvalReport
  };
}

function sink(values: string[]): { write(value: string): boolean } {
  return { write: (value) => { values.push(String(value)); return true; } };
}
