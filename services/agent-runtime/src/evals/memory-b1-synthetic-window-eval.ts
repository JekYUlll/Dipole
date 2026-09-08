import { createHash } from "node:crypto";

import { z } from "zod";

import { canonicalJSON } from "./offline-evaluator.js";
import { evaluateMemoryB1SyntheticEval, parseMemoryB1SyntheticEvalSuite, type MemoryB1SyntheticEvalSuite } from "./memory-b1-synthetic-eval.js";

const candidateVersion = z.string().trim().min(2).max(128).regex(/^[A-Za-z0-9][A-Za-z0-9._:@/-]*$/u);
const windowSchema = z.object({
  schemaVersion: z.literal("dipole.agent.memory-b1-synthetic-window.v1"),
  candidateVersion,
  minimumRecallPassBps: z.number().int().min(0).max(10_000),
  suites: z.array(z.unknown()).min(3).max(128)
}).strict();

export interface MemoryB1SyntheticWindow {
  readonly schemaVersion: "dipole.agent.memory-b1-synthetic-window.v1";
  readonly candidateVersion: string;
  readonly minimumRecallPassBps: number;
  readonly suites: MemoryB1SyntheticEvalSuite[];
}

export interface MemoryB1SyntheticWindowReport {
  readonly schemaVersion: "dipole.agent.memory-b1-synthetic-window-report.v1";
  readonly candidateVersion: string;
  readonly windowSha256: string;
  readonly passed: boolean;
  readonly reasons: string[];
  readonly metrics: { readonly suites: number; readonly recallCases: number; readonly recalledCases: number; readonly recallPassBps: number; readonly revokeCases: number; readonly revokeBoundaryFailures: number; readonly invariantFailures: number; };
  readonly failedRecallCanarySha256: string[];
}

export function parseMemoryB1SyntheticWindow(value: unknown): MemoryB1SyntheticWindow {
  const decoded = typeof value === "string" ? JSON.parse(value) as unknown : value;
  const parsed = windowSchema.parse(decoded);
  const suites = parsed.suites.map(parseMemoryB1SyntheticEvalSuite);
  if (suites.some(suite => suite.candidateVersion !== parsed.candidateVersion)) throw new Error("B1 synthetic window candidate version drift");
  const recallCanaries = suites.flatMap(suite => suite.cases.filter(item => item.recallExpectation === "required").map(item => item.canarySha256));
  if (recallCanaries.length < 3 || new Set(recallCanaries).size !== recallCanaries.length) throw new Error("B1 synthetic window recall canaries must be unique and contain at least three samples");
  return { ...parsed, suites };
}

export function evaluateMemoryB1SyntheticWindow(value: unknown): MemoryB1SyntheticWindowReport {
  const window = parseMemoryB1SyntheticWindow(value);
  let recallCases = 0;
  let recalledCases = 0;
  let revokeCases = 0;
  let revokeBoundaryFailures = 0;
  let invariantFailures = 0;
  const failedRecallCanarySha256: string[] = [];
  for (const suite of window.suites) {
    const report = evaluateMemoryB1SyntheticEval(suite);
    const failures = new Map(report.failures.map(item => [item.caseId, item.reasons]));
    for (const expected of suite.cases) {
      const reasons = failures.get(expected.caseId) ?? [];
      if (expected.recallExpectation === "required") {
        recallCases += 1;
        if (reasons.length === 0) recalledCases += 1;
        else failedRecallCanarySha256.push(expected.canarySha256);
      }
      if (expected.expectedMemoryLineageCount === 0) {
        revokeCases += 1;
        if (reasons.length > 0) revokeBoundaryFailures += 1;
      }
      if (reasons.some(reason => reason !== "synthetic_canary_not_recalled")) invariantFailures += 1;
    }
  }
  const recallPassBps = Math.floor(recalledCases * 10_000 / recallCases);
  const reasons: string[] = [];
  if (invariantFailures > 0) reasons.push("suite_invariant_failure");
  if (revokeBoundaryFailures > 0) reasons.push("revoke_boundary_failure");
  if (recallPassBps < window.minimumRecallPassBps) reasons.push("recall_below_minimum");
  return {
    schemaVersion: "dipole.agent.memory-b1-synthetic-window-report.v1",
    candidateVersion: window.candidateVersion,
    windowSha256: createHash("sha256").update(canonicalJSON(window)).digest("hex"),
    passed: reasons.length === 0,
    reasons,
    metrics: { suites: window.suites.length, recallCases, recalledCases, recallPassBps, revokeCases, revokeBoundaryFailures, invariantFailures },
    failedRecallCanarySha256
  };
}
