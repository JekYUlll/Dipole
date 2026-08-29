import { describe, expect, it } from "vitest";
import { createHash } from "node:crypto";

import {
  evaluateMemoryReviewedCorpus,
  parseMemoryReviewedCorpus,
  parseMemoryReviewedCorpusReview,
  type MemoryReviewedCorpus,
  type MemoryReviewedCorpusReview
} from "./memory-reviewed-corpus.js";
import { canonicalJSON } from "./offline-evaluator.js";

function corpus(): MemoryReviewedCorpus {
  const value: MemoryReviewedCorpus = {
    schemaVersion: "dipole.agent.memory-reviewed-corpus.v1",
    corpusId: "memory-corpus:example",
    cases: [
      { caseId: "decision-1", candidateType: "message", resourceType: "conversation", evidenceCount: 2, contentSha256: "a".repeat(64), goldPromotable: true },
      { caseId: "noise-1", candidateType: "message", resourceType: "conversation", evidenceCount: 1, contentSha256: "b".repeat(64), goldPromotable: false },
      { caseId: "risk-1", candidateType: "reflection", resourceType: "conversation", evidenceCount: 3, contentSha256: "c".repeat(64), goldPromotable: true }
    ],
    sha256: ""
  };
  value.sha256 = createHash("sha256").update(canonicalJSON({ schemaVersion: value.schemaVersion, corpusId: value.corpusId, cases: value.cases })).digest("hex");
  return value;
}

function reviewed(): MemoryReviewedCorpusReview {
  return {
    schemaVersion: "dipole.agent.memory-reviewed-corpus-review.v1", corpusSha256: "", minimumAgreementBps: 10_000,
    reviews: [
      { reviewId: "review:one", reviewerId: "reviewer:one", labels: [{ caseId: "decision-1", promotable: true }, { caseId: "noise-1", promotable: false }, { caseId: "risk-1", promotable: true }] },
      { reviewId: "review:two", reviewerId: "reviewer:two", labels: [{ caseId: "decision-1", promotable: true }, { caseId: "noise-1", promotable: false }, { caseId: "risk-1", promotable: true }] }
    ]
  };
}

describe("Memory reviewed corpus", () => {
  it("requires exact two-reviewer agreement and returns a low-sensitivity report", () => {
    const source = corpus();
    const parsed = parseMemoryReviewedCorpus(source);
    const review = reviewed();
    review.corpusSha256 = parsed.sha256;
    const report = evaluateMemoryReviewedCorpus(parsed, review);

    expect(report).toMatchObject({ passed: true, metrics: { totalCases: 3, agreedCases: 3, disagreedCases: 0, adjudicatedCases: 0 } });
    expect(report).not.toHaveProperty("reviewerIds");
    expect(JSON.stringify(report)).not.toMatch(/decision-1|reviewer:one|contentSha256/i);
  });

  it("requires an independent adjudicator for disagreement and rejects gold-label drift", () => {
    const parsed = parseMemoryReviewedCorpus(corpus());
    const review = reviewed();
    review.corpusSha256 = parsed.sha256;
    review.reviews[1]!.labels[0] = { caseId: "decision-1", promotable: false };
    expect(() => evaluateMemoryReviewedCorpus(parsed, review)).toThrow(/adjudication/i);
    review.minimumAgreementBps = 0;
    review.adjudication = { reviewId: "review:judge", adjudicatorId: "reviewer:judge", labels: [{ caseId: "decision-1", promotable: true }] };
    expect(evaluateMemoryReviewedCorpus(parsed, review)).toMatchObject({ passed: true, metrics: { disagreedCases: 1, adjudicatedCases: 1 } });
    review.adjudication.labels[0] = { caseId: "decision-1", promotable: false };
    expect(evaluateMemoryReviewedCorpus(parsed, review)).toMatchObject({ passed: false, reasons: ["final_label_mismatch"] });
  });

  it("fails closed on incomplete coverage, duplicate reviewers, and corpus hash drift", () => {
    const parsed = parseMemoryReviewedCorpus(corpus());
    const review = reviewed();
    review.corpusSha256 = parsed.sha256;
    review.reviews[0]!.labels = review.reviews[0]!.labels.slice(1);
    expect(() => evaluateMemoryReviewedCorpus(parsed, review)).toThrow(/exactly/i);
    const duplicate = reviewed();
    duplicate.corpusSha256 = parsed.sha256;
    duplicate.reviews[1]!.reviewerId = duplicate.reviews[0]!.reviewerId;
    expect(() => parseMemoryReviewedCorpusReview(duplicate)).toThrow(/reviewer/i);
    expect(() => evaluateMemoryReviewedCorpus(parsed, { ...reviewed(), corpusSha256: "f".repeat(64) })).toThrow(/SHA-256/i);
  });
});
