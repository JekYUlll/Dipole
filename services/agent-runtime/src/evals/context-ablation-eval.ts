import { createHash } from "node:crypto";

import { z } from "zod";

import { canonicalJSON } from "./offline-evaluator.js";

const id = z.string().trim().min(2).max(128).regex(/^[A-Za-z0-9][A-Za-z0-9._:-]*$/);
const condition = z.enum(["baseline", "retrieval", "memory"]);
const metrics = z.object({
  modelCalls: z.number().int().nonnegative(), toolCalls: z.number().int().nonnegative(),
  totalTokens: z.number().int().nonnegative(), totalCostMicrousd: z.number().int().nonnegative(), latencyMs: z.number().int().nonnegative()
}).strict();
const result = z.object({
  condition, outputIds: z.array(id).max(32), evidenceIds: z.array(id).max(64),
  allowed: z.boolean(), metrics
}).strict();
const suite = z.object({
  schemaVersion: z.literal("dipole.agent.context-ablation-eval.v1"), candidateVersion: z.string().min(2).max(128),
  cases: z.array(z.object({ caseId: id, requiredOutputIds: z.array(id).min(1).max(32), relevantEvidenceIds: z.array(id).min(1).max(64), results: z.array(result).length(3) }).strict()).min(1).max(256)
}).strict().superRefine((value, ctx) => {
  if (new Set(value.cases.map(item => item.caseId)).size !== value.cases.length) ctx.addIssue({ code: "custom", message: "case IDs must be unique" });
  for (const item of value.cases) if (new Set(item.results.map(entry => entry.condition)).size !== 3) ctx.addIssue({ code: "custom", message: "each case requires all three conditions" });
});

export type ContextAblationEvalSuite = z.infer<typeof suite>;
export function parseContextAblationEvalSuite(value: unknown): ContextAblationEvalSuite { return suite.parse(value); }
export function evaluateContextAblation(raw: unknown) {
  const value = suite.parse(raw);
  const aggregates = Object.fromEntries(condition.options.map(name => [name, aggregate(value, name)]));
  return { schemaVersion: "dipole.agent.context-ablation-report.v1", candidateVersion: value.candidateVersion,
    suiteSha256: createHash("sha256").update(canonicalJSON(value)).digest("hex"), caseCount: value.cases.length, conditions: aggregates };
}
function aggregate(value: ContextAblationEvalSuite, name: z.infer<typeof condition>) {
  let outputHits = 0; let evidenceHits = 0; let allowed = 0; const totals = { modelCalls: 0, toolCalls: 0, totalTokens: 0, totalCostMicrousd: 0, latencyMs: 0 };
  for (const item of value.cases) {
    const observed = item.results.find(entry => entry.condition === name)!;
    if (item.requiredOutputIds.every(output => observed.outputIds.includes(output))) outputHits += 1;
    if (item.relevantEvidenceIds.every(evidence => observed.evidenceIds.includes(evidence))) evidenceHits += 1;
    if (observed.allowed) allowed += 1;
    for (const key of Object.keys(totals) as Array<keyof typeof totals>) totals[key] += observed.metrics[key];
  }
  return { completedCases: outputHits, evidenceRecallCases: evidenceHits, permissionSafeCases: allowed, metrics: totals };
}
