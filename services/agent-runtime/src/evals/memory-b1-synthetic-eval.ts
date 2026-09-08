import { createHash } from "node:crypto";

import { z } from "zod";

import { canonicalJSON } from "./offline-evaluator.js";

const id = z.string().trim().min(2).max(128).regex(/^[a-z0-9][a-z0-9._:-]*$/u);
const sha256 = z.string().regex(/^[a-f0-9]{64}$/u);
const candidateVersion = z.string().trim().min(2).max(128).regex(/^[A-Za-z0-9][A-Za-z0-9._:@/-]*$/u);
const recallExpectation = z.enum(["required", "not_evaluated"]);
const manifestCase = z.object({
  caseId: id, canarySha256: sha256, expectedMemoryLineageCount: z.number().int().min(0).max(1),
  recallExpectation
}).strict();
const observationCase = z.object({
  caseId: id, canarySha256: sha256, taskSha256: sha256, replySha256: sha256,
  taskCompleted: z.boolean(), modelCallCount: z.number().int().min(0).max(8),
  memoryLineageCount: z.number().int().min(0).max(8), responseContainsCanary: z.boolean()
}).strict();
const suiteSchema = z.object({
  schemaVersion: z.literal("dipole.agent.memory-b1-synthetic-eval.v1"), candidateVersion,
  minimumPassBps: z.number().int().min(0).max(10_000),
  cases: z.array(manifestCase).min(2).max(256), observations: z.array(observationCase).min(2).max(256)
}).strict().superRefine((value, context) => {
  if (new Set(value.cases.map(item => item.caseId)).size !== value.cases.length) context.addIssue({ code: "custom", message: "B1 synthetic Eval case IDs must be unique" });
  if (new Set(value.observations.map(item => item.caseId)).size !== value.observations.length) context.addIssue({ code: "custom", message: "B1 synthetic Eval observation case IDs must be unique" });
  if (new Set(value.observations.map(item => item.taskSha256)).size !== value.observations.length) context.addIssue({ code: "custom", message: "B1 synthetic Eval Task hashes must be unique" });
});

export type MemoryB1SyntheticEvalSuite = z.infer<typeof suiteSchema>;

export interface MemoryB1SyntheticEvalReport {
  readonly schemaVersion: "dipole.agent.memory-b1-synthetic-eval-report.v1";
  readonly candidateVersion: string;
  readonly suiteSha256: string;
  readonly passed: boolean;
  readonly metrics: { totalCases: number; passedCases: number; passBps: number; minimumPassBps: number; recallCases: number; revokedCases: number; };
  readonly failures: Array<{ readonly caseId: string; readonly reasons: string[] }>;
}

export function parseMemoryB1SyntheticEvalSuite(value: unknown): MemoryB1SyntheticEvalSuite {
  return suiteSchema.parse(typeof value === "string" ? JSON.parse(value) as unknown : value);
}

export function evaluateMemoryB1SyntheticEval(value: unknown): MemoryB1SyntheticEvalReport {
  const suite = parseMemoryB1SyntheticEvalSuite(value);
  if (suite.cases.length !== suite.observations.length) throw new Error("B1 synthetic Eval cases and observations must have equal size");
  const observations = new Map(suite.observations.map(item => [item.caseId, item]));
  const failures = suite.cases.flatMap(expected => {
    const observed = observations.get(expected.caseId);
    if (observed === undefined) return [{ caseId: expected.caseId, reasons: ["missing_observation"] }];
    const reasons: string[] = [];
    if (observed.canarySha256 !== expected.canarySha256) reasons.push("canary_hash_mismatch");
    if (!observed.taskCompleted) reasons.push("task_not_completed");
    if (observed.modelCallCount !== 1) reasons.push("model_call_count_mismatch");
    if (observed.memoryLineageCount !== expected.expectedMemoryLineageCount) reasons.push("memory_lineage_count_mismatch");
    if (expected.recallExpectation === "required" && !observed.responseContainsCanary) reasons.push("synthetic_canary_not_recalled");
    return reasons.length === 0 ? [] : [{ caseId: expected.caseId, reasons }];
  });
  const passedCases = suite.cases.length - failures.length;
  const passBps = Math.floor(passedCases * 10_000 / suite.cases.length);
  return {
    schemaVersion: "dipole.agent.memory-b1-synthetic-eval-report.v1", candidateVersion: suite.candidateVersion,
    suiteSha256: createHash("sha256").update(canonicalJSON(suite)).digest("hex"),
    passed: failures.length === 0 && passBps >= suite.minimumPassBps,
    metrics: {
      totalCases: suite.cases.length, passedCases, passBps, minimumPassBps: suite.minimumPassBps,
      recallCases: suite.cases.filter(item => item.recallExpectation === "required").length,
      revokedCases: suite.cases.filter(item => item.expectedMemoryLineageCount === 0).length
    },
    failures
  };
}
