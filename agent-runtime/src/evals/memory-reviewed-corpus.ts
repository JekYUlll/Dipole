import { createHash } from "node:crypto";

import { z } from "zod";

import { canonicalJSON } from "./offline-evaluator.js";

const identifier = z.string().trim().min(2).max(128).regex(/^[a-z0-9][a-z0-9._:-]*$/u);
const sha256 = z.string().regex(/^[a-f0-9]{64}$/u);
const candidateType = z.enum(["message", "reflection"]);
const corpusCase = z.object({
  caseId: identifier, candidateType, resourceType: identifier,
  evidenceCount: z.number().int().min(1).max(128), contentSha256: sha256, goldPromotable: z.boolean()
}).strict();
const label = z.object({ caseId: identifier, promotable: z.boolean() }).strict();
const labels = z.array(label).min(1).max(100_000).superRefine((items, context) => {
  if (new Set(items.map(item => item.caseId)).size !== items.length) context.addIssue({ code: "custom", message: "memory corpus label case IDs must be unique" });
});
const reviewer = z.object({ reviewId: identifier, reviewerId: identifier, labels }).strict();
const adjudication = z.object({ reviewId: identifier, adjudicatorId: identifier, labels }).strict();
const corpusSchema = z.object({
  schemaVersion: z.literal("dipole.agent.memory-reviewed-corpus.v1"), corpusId: identifier,
  cases: z.array(corpusCase).min(1).max(100_000), sha256
}).strict().superRefine((value, context) => {
  if (new Set(value.cases.map(item => item.caseId)).size !== value.cases.length) context.addIssue({ code: "custom", message: "memory corpus case IDs must be unique" });
});
const reviewSchema = z.object({
  schemaVersion: z.literal("dipole.agent.memory-reviewed-corpus-review.v1"), corpusSha256: sha256,
  minimumAgreementBps: z.number().int().min(0).max(10_000), reviews: z.tuple([reviewer, reviewer]), adjudication: adjudication.optional()
}).strict().superRefine((value, context) => {
  if (value.reviews[0].reviewerId === value.reviews[1].reviewerId) context.addIssue({ code: "custom", message: "memory corpus reviewer identities must be distinct" });
  const reviewIds = [...value.reviews.map(item => item.reviewId), ...(value.adjudication === undefined ? [] : [value.adjudication.reviewId])];
  if (new Set(reviewIds).size !== reviewIds.length) context.addIssue({ code: "custom", message: "memory corpus review IDs must be unique" });
  if (value.adjudication !== undefined && value.reviews.some(item => item.reviewerId === value.adjudication!.adjudicatorId)) {
    context.addIssue({ code: "custom", message: "memory corpus adjudicator must be independent" });
  }
});

export interface MemoryReviewedCorpus {
  schemaVersion: "dipole.agent.memory-reviewed-corpus.v1";
  corpusId: string;
  cases: Array<z.infer<typeof corpusCase>>;
  sha256: string;
  review?: MemoryReviewedCorpusReview;
}

export type MemoryReviewedCorpusReview = z.infer<typeof reviewSchema>;

export interface MemoryReviewedCorpusReport {
  readonly schemaVersion: "dipole.agent.memory-reviewed-corpus-report.v1";
  readonly corpusSha256: string;
  readonly reviewSha256: string;
  readonly finalLabelsSha256: string;
  readonly passed: boolean;
  readonly reasons: string[];
  readonly metrics: {
    totalCases: number;
    agreedCases: number;
    disagreedCases: number;
    agreementBps: number;
    minimumAgreementBps: number;
    adjudicatedCases: number;
    promotableCases: number;
  };
  readonly disagreementCount: number;
  readonly finalLabelMismatchCount: number;
}

export function parseMemoryReviewedCorpus(value: unknown): MemoryReviewedCorpus {
  const decoded = typeof value === "string" ? JSON.parse(value) as unknown : value;
  const parsed = corpusSchema.parse(decoded);
  const expected = digest({ schemaVersion: parsed.schemaVersion, corpusId: parsed.corpusId, cases: parsed.cases });
  if (parsed.sha256 !== expected) throw new Error("memory corpus SHA-256 is invalid");
  return parsed as MemoryReviewedCorpus;
}

export function parseMemoryReviewedCorpusReview(value: unknown): MemoryReviewedCorpusReview {
  const decoded = typeof value === "string" ? JSON.parse(value) as unknown : value;
  const parsed = reviewSchema.parse(decoded);
  const normalizedReviews = parsed.reviews
    .map(item => ({ ...item, labels: sortLabels(item.labels) }))
    .sort((left, right) => compareASCII(`${left.reviewerId}\u0000${left.reviewId}`, `${right.reviewerId}\u0000${right.reviewId}`));
  return {
    ...parsed,
    reviews: normalizedReviews as MemoryReviewedCorpusReview["reviews"],
    ...(parsed.adjudication === undefined ? {} : { adjudication: { ...parsed.adjudication, labels: sortLabels(parsed.adjudication.labels) } })
  };
}

export function evaluateMemoryReviewedCorpus(corpus: MemoryReviewedCorpus, rawReview: MemoryReviewedCorpusReview): MemoryReviewedCorpusReport {
  const validatedCorpus = parseMemoryReviewedCorpus(corpus);
  const review = parseMemoryReviewedCorpusReview(rawReview);
  if (review.corpusSha256 !== validatedCorpus.sha256) throw new Error("memory corpus review SHA-256 does not match the corpus");
  const caseIds = validatedCorpus.cases.map(item => item.caseId);
  for (const item of review.reviews) requireExactCases(caseIds, item.labels.map(value => value.caseId));
  const first = new Map(review.reviews[0].labels.map(item => [item.caseId, item.promotable]));
  const second = new Map(review.reviews[1].labels.map(item => [item.caseId, item.promotable]));
  const disagreementIds = caseIds.filter(id => first.get(id) !== second.get(id));
  if (disagreementIds.length === 0 && review.adjudication !== undefined) throw new Error("memory corpus adjudication must be absent when reviewers agree");
  if (disagreementIds.length > 0 && review.adjudication === undefined) throw new Error("memory corpus adjudication is required for every disagreement");
  if (review.adjudication !== undefined) requireExactCases(disagreementIds, review.adjudication.labels.map(value => value.caseId));
  const adjudicated = new Map(review.adjudication?.labels.map(item => [item.caseId, item.promotable]) ?? []);
  const finalLabels = caseIds.map(caseId => ({ caseId, promotable: first.get(caseId) === second.get(caseId) ? first.get(caseId)! : adjudicated.get(caseId)! }));
  const gold = new Map(validatedCorpus.cases.map(item => [item.caseId, item.goldPromotable]));
  const mismatchCount = finalLabels.filter(item => item.promotable !== gold.get(item.caseId)).length;
  const agreementBps = Math.floor((caseIds.length - disagreementIds.length) * 10_000 / caseIds.length);
  const reasons: string[] = [];
  if (agreementBps < review.minimumAgreementBps) reasons.push("agreement_below_minimum");
  if (mismatchCount > 0) reasons.push("final_label_mismatch");
  return {
    schemaVersion: "dipole.agent.memory-reviewed-corpus-report.v1", corpusSha256: validatedCorpus.sha256,
    reviewSha256: digest(review), finalLabelsSha256: digest(finalLabels), passed: reasons.length === 0, reasons,
    metrics: {
      totalCases: caseIds.length, agreedCases: caseIds.length - disagreementIds.length, disagreedCases: disagreementIds.length,
      agreementBps, minimumAgreementBps: review.minimumAgreementBps, adjudicatedCases: adjudicated.size,
      promotableCases: finalLabels.filter(item => item.promotable).length
    }, disagreementCount: disagreementIds.length, finalLabelMismatchCount: mismatchCount
  };
}

export function memoryReviewedCorpusReviewSha256(rawReview: MemoryReviewedCorpusReview): string {
  return digest(parseMemoryReviewedCorpusReview(rawReview));
}

function requireExactCases(expected: readonly string[], observed: readonly string[]): void {
  if (expected.length !== observed.length || expected.some((value, index) => value !== observed[index])) throw new Error("memory corpus labels must cover exactly the required cases");
}

function sortLabels(items: readonly { caseId: string; promotable: boolean }[]) {
  return [...items].sort((left, right) => compareASCII(left.caseId, right.caseId));
}

function compareASCII(left: string, right: string): number {
  return left < right ? -1 : left > right ? 1 : 0;
}

function digest(value: unknown): string {
  return createHash("sha256").update(canonicalJSON(value)).digest("hex");
}
