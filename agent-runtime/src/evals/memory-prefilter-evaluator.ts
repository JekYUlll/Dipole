import { createHash } from "node:crypto";

import { z } from "zod";

import { canonicalJSON } from "./offline-evaluator.js";
import { parseMemoryReviewedCorpus, type MemoryReviewedCorpus } from "./memory-reviewed-corpus.js";

const identifier = z.string().trim().min(2).max(128).regex(/^[a-z0-9][a-z0-9._:-]*$/u);
const revision = z.string().trim().min(2).max(128).regex(/^[A-Za-z0-9][A-Za-z0-9._:@/-]*$/u);
const sha256 = z.string().regex(/^[a-f0-9]{64}$/u);
const policySchema = z.object({
  minimumPrecisionBps: z.number().int().min(0).max(10_000),
  minimumRecallBps: z.number().int().min(0).max(10_000),
  maximumP95LatencyMicros: z.number().int().positive().max(60_000_000),
  maximumMeanCostMicrousd: z.number().int().nonnegative().max(1_000_000_000)
}).strict();
const candidateSchema = z.object({
  id: identifier,
  kind: z.enum(["embedding", "small_model"]),
  revision,
  configurationSha256: sha256,
  decisionThresholdBps: z.number().int().min(0).max(10_000)
}).strict();
const decisionSchema = z.object({
  caseId: identifier,
  selected: z.boolean(),
  scoreBps: z.number().int().min(0).max(10_000),
  latencyMicros: z.number().int().nonnegative().max(60_000_000),
  costMicrousd: z.number().int().nonnegative().max(1_000_000_000)
}).strict();
const evidenceSchema = z.object({
  schemaVersion: z.literal("dipole.agent.memory-prefilter-evidence.v1"),
  corpusSha256: sha256,
  candidate: candidateSchema,
  decisions: z.array(decisionSchema).min(1).max(100_000)
}).strict().superRefine((value, context) => {
  const ids = value.decisions.map(item => item.caseId);
  if (new Set(ids).size !== ids.length) context.addIssue({ code: "custom", message: "memory prefilter decision case IDs must be unique" });
  if (value.decisions.some(item => item.selected !== (item.scoreBps >= value.candidate.decisionThresholdBps))) context.addIssue({ code: "custom", message: "memory prefilter decision conflicts with its score threshold" });
});

export type MemoryPrefilterPolicy = z.infer<typeof policySchema>;
export type MemoryPrefilterEvidence = z.infer<typeof evidenceSchema>;
export interface MemoryPrefilterReport {
  readonly schemaVersion: "dipole.agent.memory-prefilter-report.v1";
  readonly corpusSha256: string;
  readonly evidenceSha256: string;
  readonly candidate: MemoryPrefilterEvidence["candidate"];
  readonly passed: boolean;
  readonly reasons: string[];
  readonly confusion: { readonly truePositive: number; readonly trueNegative: number; readonly falsePositive: number; readonly falseNegative: number };
  readonly metrics: { readonly precisionBps: number; readonly recallBps: number; readonly p95LatencyMicros: number; readonly meanCostMicrousd: number; readonly totalCostMicrousd: number };
}

export function parseMemoryPrefilterEvidence(value: unknown): MemoryPrefilterEvidence {
  const decoded = typeof value === "string" ? JSON.parse(value) as unknown : value;
  const parsed = evidenceSchema.parse(decoded);
  return { ...parsed, decisions: [...parsed.decisions].sort((left, right) => compareASCII(left.caseId, right.caseId)) };
}

export function parseMemoryPrefilterPolicy(value: unknown): MemoryPrefilterPolicy {
  const decoded = typeof value === "string" ? JSON.parse(value) as unknown : value;
  return policySchema.parse(decoded);
}

export function evaluateMemoryPrefilter(corpus: MemoryReviewedCorpus, rawEvidence: MemoryPrefilterEvidence, rawPolicy: MemoryPrefilterPolicy): MemoryPrefilterReport {
  const validatedCorpus = parseMemoryReviewedCorpus(corpus);
  const evidence = parseMemoryPrefilterEvidence(rawEvidence);
  const policy = parseMemoryPrefilterPolicy(rawPolicy);
  if (validatedCorpus.sha256 !== evidence.corpusSha256) throw new Error("memory prefilter evidence corpus SHA-256 does not match the reviewed corpus");
  const expectedIds = validatedCorpus.cases.map(item => item.caseId);
  const observedIds = evidence.decisions.map(item => item.caseId);
  if (!equalLists(expectedIds, observedIds)) throw new Error("memory prefilter evidence must contain exactly one decision for every corpus case");
  const decisions = new Map(evidence.decisions.map(item => [item.caseId, item]));
  let truePositive = 0; let trueNegative = 0; let falsePositive = 0; let falseNegative = 0;
  for (const item of validatedCorpus.cases) {
    const selected = decisions.get(item.caseId)!.selected;
    if (item.goldPromotable && selected) truePositive += 1;
    else if (!item.goldPromotable && !selected) trueNegative += 1;
    else if (selected) falsePositive += 1;
    else falseNegative += 1;
  }
  const precisionBps = basisPoints(truePositive, truePositive + falsePositive);
  const recallBps = basisPoints(truePositive, truePositive + falseNegative);
  const latencies = evidence.decisions.map(item => item.latencyMicros).sort((a, b) => a - b);
  const p95LatencyMicros = latencies[Math.ceil(latencies.length * 0.95) - 1]!;
  const totalCostMicrousd = evidence.decisions.reduce((sum, item) => sum + item.costMicrousd, 0);
  const meanCostMicrousd = Math.ceil(totalCostMicrousd / evidence.decisions.length);
  const reasons: string[] = [];
  if (precisionBps < policy.minimumPrecisionBps) reasons.push("precision_below_minimum");
  if (recallBps < policy.minimumRecallBps) reasons.push("recall_below_minimum");
  if (p95LatencyMicros > policy.maximumP95LatencyMicros) reasons.push("p95_latency_exceeded");
  if (meanCostMicrousd > policy.maximumMeanCostMicrousd) reasons.push("mean_cost_exceeded");
  reasons.sort();
  return {
    schemaVersion: "dipole.agent.memory-prefilter-report.v1", corpusSha256: validatedCorpus.sha256,
    evidenceSha256: digest(evidence), candidate: evidence.candidate, passed: reasons.length === 0, reasons,
    confusion: { truePositive, trueNegative, falsePositive, falseNegative },
    metrics: { precisionBps, recallBps, p95LatencyMicros, meanCostMicrousd, totalCostMicrousd }
  };
}

function basisPoints(numerator: number, denominator: number): number { return denominator === 0 ? 0 : Math.floor(numerator * 10_000 / denominator); }
function equalLists(left: readonly string[], right: readonly string[]): boolean { return left.length === right.length && left.every((value, index) => value === right[index]); }
function compareASCII(left: string, right: string): number { return left < right ? -1 : left > right ? 1 : 0; }
function digest(value: unknown): string { return createHash("sha256").update(canonicalJSON(value)).digest("hex"); }
