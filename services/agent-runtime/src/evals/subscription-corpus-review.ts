import { createHash } from "node:crypto";

import { z } from "zod";

import { canonicalJSON } from "./offline-evaluator.js";
import { parseSubscriptionPrefilterCorpus, type SubscriptionPrefilterCorpus } from "./subscription-prefilter-evaluator.js";

const identifier = z.string().trim().min(2).max(128).regex(/^[a-z0-9][a-z0-9._:-]*$/u);
const sha256 = z.string().regex(/^[a-f0-9]{64}$/u);
const label = z.object({ caseId: identifier, relevant: z.boolean() }).strict();
const labels = z.array(label).min(1).max(10_000).superRefine((items, context) => {
  if (new Set(items.map(item => item.caseId)).size !== items.length) context.addIssue({ code: "custom", message: "label case IDs must be unique" });
});
const review = z.object({ reviewId: identifier, reviewerId: identifier, labels }).strict();
const adjudication = z.object({ reviewId: identifier, adjudicatorId: identifier, labels }).strict();
const reviewSchema = z.object({
  schemaVersion: z.literal("dipole.agent.subscription-corpus-review.v1"),
  corpusSha256: sha256,
  minimumAgreementBps: z.number().int().min(0).max(10_000),
  reviews: z.tuple([review, review]),
  adjudication: adjudication.optional()
}).strict().superRefine((value, context) => {
  const reviewerIds = value.reviews.map(item => item.reviewerId);
  const reviewIds = [...value.reviews.map(item => item.reviewId), ...(value.adjudication === undefined ? [] : [value.adjudication.reviewId])];
  if (new Set(reviewerIds).size !== reviewerIds.length) context.addIssue({ code: "custom", message: "reviewer identities must be distinct" });
  if (new Set(reviewIds).size !== reviewIds.length) context.addIssue({ code: "custom", message: "review IDs must be unique" });
  if (value.adjudication !== undefined && reviewerIds.includes(value.adjudication.adjudicatorId)) {
    context.addIssue({ code: "custom", message: "adjudicator identity must be distinct from reviewers" });
  }
});

export type SubscriptionCorpusReview = z.infer<typeof reviewSchema>;

export interface SubscriptionCorpusReviewReport {
  readonly schemaVersion: "dipole.agent.subscription-corpus-review-report.v1";
  readonly corpusSha256: string;
  readonly reviewSha256: string;
  readonly finalLabelsSha256: string;
  readonly passed: boolean;
  readonly reasons: string[];
  readonly metrics: { totalCases: number; agreedCases: number; disagreedCases: number; agreementBps: number; minimumAgreementBps: number; adjudicatedCases: number };
  readonly disagreementCaseIds: string[];
  readonly finalLabelMismatchCaseIds: string[];
}

export function parseSubscriptionCorpusReview(value: unknown): SubscriptionCorpusReview {
  const decoded = typeof value === "string" ? JSON.parse(value) as unknown : value;
  const parsed = reviewSchema.parse(decoded);
  const normalizedReviews = parsed.reviews
    .map(item => ({ ...item, labels: sortLabels(item.labels) }))
    .sort((left, right) => compareASCII(`${left.reviewerId}\u0000${left.reviewId}`, `${right.reviewerId}\u0000${right.reviewId}`));
  return {
    ...parsed,
    reviews: normalizedReviews as SubscriptionCorpusReview["reviews"],
    ...(parsed.adjudication === undefined ? {} : { adjudication: { ...parsed.adjudication, labels: sortLabels(parsed.adjudication.labels) } })
  };
}

export function evaluateSubscriptionCorpusReview(corpus: SubscriptionPrefilterCorpus, rawReview: SubscriptionCorpusReview): SubscriptionCorpusReviewReport {
  const validatedCorpus = parseSubscriptionPrefilterCorpus(stripCorpusHash(corpus));
  const reviewed = parseSubscriptionCorpusReview(rawReview);
  if (reviewed.corpusSha256 !== validatedCorpus.sha256) throw new Error("review corpus SHA-256 does not match the labeled corpus");
  const caseIds = validatedCorpus.cases.map(item => item.id);
  for (const item of reviewed.reviews) requireExactCases(caseIds, item.labels.map(value => value.caseId));
  const first = new Map(reviewed.reviews[0].labels.map(item => [item.caseId, item.relevant]));
  const second = new Map(reviewed.reviews[1].labels.map(item => [item.caseId, item.relevant]));
  const disagreementCaseIds = caseIds.filter(id => first.get(id) !== second.get(id));
  if (disagreementCaseIds.length === 0 && reviewed.adjudication !== undefined) throw new Error("adjudication must be absent when reviewers agree");
  if (disagreementCaseIds.length > 0 && reviewed.adjudication === undefined) throw new Error("adjudication is required for every reviewer disagreement");
  if (reviewed.adjudication !== undefined) requireExactCases(disagreementCaseIds, reviewed.adjudication.labels.map(item => item.caseId));
  const adjudicated = new Map(reviewed.adjudication?.labels.map(item => [item.caseId, item.relevant]) ?? []);
  const finalLabels = caseIds.map(caseId => ({ caseId, relevant: first.get(caseId) === second.get(caseId) ? first.get(caseId)! : adjudicated.get(caseId)! }));
  const expected = new Map(validatedCorpus.cases.map(item => [item.id, item.expectedRelevant]));
  const finalLabelMismatchCaseIds = finalLabels.filter(item => item.relevant !== expected.get(item.caseId)).map(item => item.caseId);
  const agreementBps = Math.floor((caseIds.length - disagreementCaseIds.length) * 10_000 / caseIds.length);
  const reasons: string[] = [];
  if (agreementBps < reviewed.minimumAgreementBps) reasons.push("agreement_below_minimum");
  if (finalLabelMismatchCaseIds.length > 0) reasons.push("final_label_mismatch");
  return {
    schemaVersion: "dipole.agent.subscription-corpus-review-report.v1", corpusSha256: validatedCorpus.sha256,
    reviewSha256: digest(reviewed), finalLabelsSha256: digest(finalLabels), passed: reasons.length === 0, reasons,
    metrics: { totalCases: caseIds.length, agreedCases: caseIds.length - disagreementCaseIds.length, disagreedCases: disagreementCaseIds.length, agreementBps, minimumAgreementBps: reviewed.minimumAgreementBps, adjudicatedCases: adjudicated.size },
    disagreementCaseIds, finalLabelMismatchCaseIds
  };
}

function requireExactCases(expected: readonly string[], observed: readonly string[]): void {
  if (expected.length !== observed.length || expected.some((value, index) => value !== observed[index])) throw new Error("labels must cover exactly the required corpus cases");
}

function sortLabels(items: readonly { caseId: string; relevant: boolean }[]) {
  return [...items].sort((left, right) => compareASCII(left.caseId, right.caseId));
}

function compareASCII(left: string, right: string): number {
  return left < right ? -1 : left > right ? 1 : 0;
}

function stripCorpusHash(corpus: SubscriptionPrefilterCorpus) {
  const { sha256: _sha256, ...record } = corpus;
  return record;
}

function digest(value: unknown): string {
  return createHash("sha256").update(canonicalJSON(value)).digest("hex");
}
