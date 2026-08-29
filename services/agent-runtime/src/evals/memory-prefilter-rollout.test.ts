import { createHash } from "node:crypto";

import { describe, expect, it } from "vitest";

import { canonicalJSON } from "./offline-evaluator.js";
import { evaluateMemoryPrefilterRollout } from "./memory-prefilter-rollout.js";
import { parseMemoryPrefilterEvidence, type MemoryPrefilterPolicy } from "./memory-prefilter-evaluator.js";
import { parseMemoryReviewedCorpus, type MemoryReviewedCorpusReview } from "./memory-reviewed-corpus.js";

const record = {
  schemaVersion: "dipole.agent.memory-reviewed-corpus.v1" as const,
  corpusId: "memory-gold",
  cases: [
    { caseId: "case-a", candidateType: "message" as const, resourceType: "conversation", evidenceCount: 2, contentSha256: "a".repeat(64), goldPromotable: true },
    { caseId: "case-b", candidateType: "reflection" as const, resourceType: "project", evidenceCount: 3, contentSha256: "b".repeat(64), goldPromotable: false }
  ]
};
const corpus = parseMemoryReviewedCorpus({ ...record, sha256: createHash("sha256").update(canonicalJSON(record)).digest("hex") });
const review: MemoryReviewedCorpusReview = {
  schemaVersion: "dipole.agent.memory-reviewed-corpus-review.v1", corpusSha256: corpus.sha256, minimumAgreementBps: 10_000,
  reviews: [
    { reviewId: "review-a", reviewerId: "reviewer-a", labels: [{ caseId: "case-a", promotable: true }, { caseId: "case-b", promotable: false }] },
    { reviewId: "review-b", reviewerId: "reviewer-b", labels: [{ caseId: "case-a", promotable: true }, { caseId: "case-b", promotable: false }] }
  ]
};
const policy: MemoryPrefilterPolicy = { minimumPrecisionBps: 10_000, minimumRecallBps: 10_000, maximumP95LatencyMicros: 2_000, maximumMeanCostMicrousd: 20 };
const evidence = parseMemoryPrefilterEvidence({
  schemaVersion: "dipole.agent.memory-prefilter-evidence.v1", corpusSha256: corpus.sha256,
  candidate: { id: "embedding:v1", kind: "embedding", revision: "model@1", configurationSha256: "e".repeat(64), decisionThresholdBps: 7_500 },
  decisions: [
    { caseId: "case-a", selected: true, scoreBps: 9_000, latencyMicros: 1_000, costMicrousd: 10 },
    { caseId: "case-b", selected: false, scoreBps: 1_000, latencyMicros: 1_100, costMicrousd: 10 }
  ]
});

describe("memory prefilter rollout decision", () => {
  it("recomputes review and candidate gates before eligibility", () => {
    expect(evaluateMemoryPrefilterRollout(corpus, review, evidence, policy)).toMatchObject({ decision: "eligible", reasons: [], metrics: { agreementBps: 10_000, precisionBps: 10_000 } });
  });

  it("blocks when the candidate evidence misses a gate", () => {
    const blocked = evaluateMemoryPrefilterRollout(corpus, review, { ...evidence, decisions: evidence.decisions.map(item => ({ ...item, selected: true, scoreBps: 9_000 })) }, policy);
    expect(blocked).toMatchObject({ decision: "blocked", reasons: ["candidate_prefilter_blocked"] });
  });

  it("blocks review drift instead of trusting caller supplied reports", () => {
    const drifted = { ...review, reviews: review.reviews.map(item => ({ ...item, labels: item.labels.map(label => ({ ...label, promotable: !label.promotable })) })) } as MemoryReviewedCorpusReview;
    expect(evaluateMemoryPrefilterRollout(corpus, drifted, evidence, policy)).toMatchObject({ decision: "blocked", reasons: ["corpus_review_blocked"] });
  });
});
