import { createHash } from "node:crypto";

import { z } from "zod";

import { canonicalJSON } from "./offline-evaluator.js";

const identifier = z.string().trim().min(2).max(128).regex(/^[a-z0-9][a-z0-9._:-]*$/u);
const revision = z.string().trim().min(2).max(128).regex(/^[A-Za-z0-9][A-Za-z0-9._:@/-]*$/u);
const sha256 = z.string().regex(/^[a-f0-9]{64}$/u);

const corpusCaseSchema = z.object({
  id: identifier,
  expectedRelevant: z.boolean(),
  event: z.object({
    eventId: z.string().trim().min(1).max(128),
    eventType: z.string().trim().min(1).max(64),
    aggregateId: z.string().trim().min(1).max(128),
    occurredAt: z.iso.datetime(),
    payload: z.object({
      conversation_key: z.string().trim().min(1).max(128),
      content: z.string().max(8192)
    }).strict()
  }).strict()
}).strict();

const thresholdsSchema = z.object({
  minimumPrecisionBps: z.number().int().min(0).max(10_000),
  minimumRecallBps: z.number().int().min(0).max(10_000),
  maximumP95LatencyMicros: z.number().int().positive().max(60_000_000),
  maximumMeanCostMicrousd: z.number().int().nonnegative().max(1_000_000_000)
}).strict();

const corpusSchema = z.object({
  schemaVersion: z.literal("dipole.agent.subscription-prefilter-corpus.v1"),
  corpusId: identifier,
  revision,
  thresholds: thresholdsSchema,
  cases: z.array(corpusCaseSchema).min(2).max(10_000)
}).strict().superRefine((corpus, context) => {
  const ids = corpus.cases.map(item => item.id);
  if (new Set(ids).size !== ids.length) context.addIssue({ code: "custom", message: "corpus case IDs must be unique" });
  if (!corpus.cases.some(item => item.expectedRelevant) || !corpus.cases.some(item => !item.expectedRelevant)) {
    context.addIssue({ code: "custom", message: "corpus must contain relevant and irrelevant cases" });
  }
});

const candidateSchema = z.object({
  id: identifier,
  kind: z.enum(["rule", "embedding", "small_model"]),
  revision,
  configurationSha256: sha256,
  decisionThresholdBps: z.number().int().min(0).max(10_000).optional()
}).strict();

const decisionSchema = z.object({
  caseId: identifier,
  selected: z.boolean(),
  scoreBps: z.number().int().min(0).max(10_000).optional(),
  latencyMicros: z.number().int().nonnegative().max(60_000_000),
  costMicrousd: z.number().int().nonnegative().max(1_000_000_000)
}).strict();

const evidenceSchema = z.object({
  schemaVersion: z.literal("dipole.agent.subscription-prefilter-evidence.v1"),
  corpusSha256: sha256,
  candidate: candidateSchema,
  decisions: z.array(decisionSchema).min(1).max(10_000)
}).strict().superRefine((evidence, context) => {
  const ids = evidence.decisions.map(item => item.caseId);
  if (new Set(ids).size !== ids.length) context.addIssue({ code: "custom", message: "candidate decision case IDs must be unique" });
  if (evidence.candidate.kind === "rule") {
    if (evidence.candidate.decisionThresholdBps !== undefined || evidence.decisions.some(item => item.scoreBps !== undefined)) {
      context.addIssue({ code: "custom", message: "rule candidates cannot provide scores or a decision threshold" });
    }
    return;
  }
  const threshold = evidence.candidate.decisionThresholdBps;
  if (threshold === undefined) {
    context.addIssue({ code: "custom", message: "scored candidates require a decision threshold" });
    return;
  }
  for (const decision of evidence.decisions) {
    if (decision.scoreBps === undefined || decision.selected !== (decision.scoreBps >= threshold)) {
      context.addIssue({ code: "custom", message: "candidate decision conflicts with its score threshold" });
      return;
    }
  }
});

export type SubscriptionPrefilterCorpusRecord = z.infer<typeof corpusSchema>;
export type SubscriptionPrefilterCorpus = SubscriptionPrefilterCorpusRecord & { readonly sha256: string };
export type SubscriptionPrefilterEvidence = z.infer<typeof evidenceSchema>;

export interface SubscriptionPrefilterReport {
  readonly schemaVersion: "dipole.agent.subscription-prefilter-report.v1";
  readonly corpusId: string;
  readonly corpusRevision: string;
  readonly corpusSha256: string;
  readonly evidenceSha256: string;
  readonly candidate: SubscriptionPrefilterEvidence["candidate"];
  readonly passed: boolean;
  readonly reasons: string[];
  readonly confusion: {
    readonly truePositive: number;
    readonly trueNegative: number;
    readonly falsePositive: number;
    readonly falseNegative: number;
  };
  readonly metrics: {
    readonly precisionBps: number;
    readonly recallBps: number;
    readonly p95LatencyMicros: number;
    readonly meanCostMicrousd: number;
    readonly totalCostMicrousd: number;
  };
  readonly falsePositiveCaseIds: string[];
  readonly falseNegativeCaseIds: string[];
}

export function parseSubscriptionPrefilterCorpus(value: unknown): SubscriptionPrefilterCorpus {
  const decoded = typeof value === "string" ? JSON.parse(value) as unknown : value;
  const parsed = corpusSchema.parse(decoded);
  const normalized: SubscriptionPrefilterCorpusRecord = {
    ...parsed,
    cases: [...parsed.cases].sort((left, right) => compareASCII(left.id, right.id))
  };
  return { ...normalized, sha256: digest(normalized) };
}

export function parseSubscriptionPrefilterEvidence(value: unknown): SubscriptionPrefilterEvidence {
  const decoded = typeof value === "string" ? JSON.parse(value) as unknown : value;
  const parsed = evidenceSchema.parse(decoded);
  return { ...parsed, decisions: [...parsed.decisions].sort((left, right) => compareASCII(left.caseId, right.caseId)) };
}

export function evaluateSubscriptionPrefilter(corpus: SubscriptionPrefilterCorpus, evidence: SubscriptionPrefilterEvidence): SubscriptionPrefilterReport {
  const validatedCorpus = parseSubscriptionPrefilterCorpus(stripCorpusHash(corpus));
  const validatedEvidence = parseSubscriptionPrefilterEvidence(evidence);
  if (validatedCorpus.sha256 !== validatedEvidence.corpusSha256) throw new Error("candidate evidence corpus SHA-256 does not match the labeled corpus");
  const expectedIds = validatedCorpus.cases.map(item => item.id);
  const observedIds = validatedEvidence.decisions.map(item => item.caseId);
  if (!equalLists(expectedIds, observedIds)) throw new Error("candidate evidence must contain exactly one decision for every corpus case");

  const byCase = new Map(validatedEvidence.decisions.map(item => [item.caseId, item]));
  const falsePositiveCaseIds: string[] = [];
  const falseNegativeCaseIds: string[] = [];
  let truePositive = 0;
  let trueNegative = 0;
  for (const testCase of validatedCorpus.cases) {
    const selected = byCase.get(testCase.id)!.selected;
    if (testCase.expectedRelevant && selected) truePositive += 1;
    if (!testCase.expectedRelevant && !selected) trueNegative += 1;
    if (!testCase.expectedRelevant && selected) falsePositiveCaseIds.push(testCase.id);
    if (testCase.expectedRelevant && !selected) falseNegativeCaseIds.push(testCase.id);
  }
  const falsePositive = falsePositiveCaseIds.length;
  const falseNegative = falseNegativeCaseIds.length;
  const precisionBps = basisPoints(truePositive, truePositive + falsePositive);
  const recallBps = basisPoints(truePositive, truePositive + falseNegative);
  const latencies = validatedEvidence.decisions.map(item => item.latencyMicros).sort((left, right) => left - right);
  const p95LatencyMicros = latencies[Math.ceil(latencies.length * 0.95) - 1]!;
  const totalCostMicrousd = validatedEvidence.decisions.reduce((total, item) => total + item.costMicrousd, 0);
  const meanCostMicrousd = Math.ceil(totalCostMicrousd / validatedEvidence.decisions.length);
  const reasons: string[] = [];
  if (precisionBps < validatedCorpus.thresholds.minimumPrecisionBps) reasons.push("precision_below_minimum");
  if (recallBps < validatedCorpus.thresholds.minimumRecallBps) reasons.push("recall_below_minimum");
  if (p95LatencyMicros > validatedCorpus.thresholds.maximumP95LatencyMicros) reasons.push("p95_latency_exceeded");
  if (meanCostMicrousd > validatedCorpus.thresholds.maximumMeanCostMicrousd) reasons.push("mean_cost_exceeded");
  reasons.sort();

  return {
    schemaVersion: "dipole.agent.subscription-prefilter-report.v1",
    corpusId: validatedCorpus.corpusId, corpusRevision: validatedCorpus.revision,
    corpusSha256: validatedCorpus.sha256, evidenceSha256: digest(validatedEvidence), candidate: validatedEvidence.candidate,
    passed: reasons.length === 0, reasons,
    confusion: { truePositive, trueNegative, falsePositive, falseNegative },
    metrics: { precisionBps, recallBps, p95LatencyMicros, meanCostMicrousd, totalCostMicrousd },
    falsePositiveCaseIds, falseNegativeCaseIds
  };
}

function stripCorpusHash(corpus: SubscriptionPrefilterCorpus): SubscriptionPrefilterCorpusRecord {
  const { sha256: _sha256, ...record } = corpus;
  return record;
}

function basisPoints(numerator: number, denominator: number): number {
  return denominator === 0 ? 0 : Math.floor(numerator * 10_000 / denominator);
}

function equalLists(left: readonly string[], right: readonly string[]): boolean {
  return left.length === right.length && left.every((value, index) => value === right[index]);
}

function compareASCII(left: string, right: string): number {
  return left < right ? -1 : left > right ? 1 : 0;
}

function digest(value: unknown): string {
  return createHash("sha256").update(canonicalJSON(value)).digest("hex");
}
