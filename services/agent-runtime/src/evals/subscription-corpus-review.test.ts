import { readFile } from "node:fs/promises";

import { describe, expect, it } from "vitest";

import { parseSubscriptionPrefilterCorpus } from "./subscription-prefilter-evaluator.js";
import { evaluateSubscriptionCorpusReview, parseSubscriptionCorpusReview } from "./subscription-corpus-review.js";

describe("subscription corpus review", () => {
  it("binds two independent reviews and adjudication to the labeled corpus", () => {
    const corpus = parseSubscriptionPrefilterCorpus(corpusFixture());
    const review = parseSubscriptionCorpusReview(reviewFixture(corpus.sha256));
    const report = evaluateSubscriptionCorpusReview(corpus, review);
    expect(report).toMatchObject({
      passed: true,
      metrics: { totalCases: 2, agreedCases: 1, disagreedCases: 1, agreementBps: 5000, minimumAgreementBps: 5000, adjudicatedCases: 1 },
      disagreementCaseIds: ["positive"], reasons: []
    });
    expect(report.reviewSha256).toMatch(/^[a-f0-9]{64}$/u);
    expect(report.finalLabelsSha256).toMatch(/^[a-f0-9]{64}$/u);
    const reversed = parseSubscriptionCorpusReview({ ...reviewFixture(corpus.sha256), reviews: [...reviewFixture(corpus.sha256).reviews].reverse() });
    expect(evaluateSubscriptionCorpusReview(corpus, reversed).reviewSha256).toBe(report.reviewSha256);
    expect(JSON.stringify(report)).not.toContain("incident text");
  });

  it("keeps the language-neutral example review hashes stable", async () => {
    const corpusSource = await readFile(new URL("../../../../contracts/agent-subscription-prefilter/v1/corpus.example.json", import.meta.url), "utf8");
    const reviewSource = await readFile(new URL("../../../../contracts/agent-subscription-prefilter/v1/review.example.json", import.meta.url), "utf8");
    const report = evaluateSubscriptionCorpusReview(parseSubscriptionPrefilterCorpus(corpusSource), parseSubscriptionCorpusReview(reviewSource));
    expect(report.reviewSha256).toBe("6d9576f2f85b6f42ad8c255c0dfcce40718b5433979c4736a58358d078438f29");
    expect(report.finalLabelsSha256).toBe("a9c6f9cb253d11024c4aa69c59a4c872869635fe1cb682a6438c63343f7d893a");
  });

  it("rejects identity reuse, incomplete labels, and missing adjudication", () => {
    const corpus = parseSubscriptionPrefilterCorpus(corpusFixture());
    const base = reviewFixture(corpus.sha256);
    expect(() => parseSubscriptionCorpusReview({ ...base, reviews: [base.reviews[0], { ...base.reviews[1], reviewerId: "reviewer-a" }] })).toThrow(/reviewer/iu);
    expect(() => evaluateSubscriptionCorpusReview(corpus, parseSubscriptionCorpusReview({ ...base, reviews: [base.reviews[0], { ...base.reviews[1], labels: base.reviews[1]!.labels.slice(1) }] }))).toThrow(/exactly/iu);
    expect(() => evaluateSubscriptionCorpusReview(corpus, parseSubscriptionCorpusReview({ ...base, adjudication: undefined }))).toThrow(/adjudication/iu);
  });

  it("fails valid evidence when the final reviewed label drifts from the corpus", () => {
    const corpus = parseSubscriptionPrefilterCorpus(corpusFixture());
    const base = reviewFixture(corpus.sha256);
    const review = parseSubscriptionCorpusReview({ ...base, adjudication: { ...base.adjudication, labels: [{ caseId: "positive", relevant: false }] } });
    expect(evaluateSubscriptionCorpusReview(corpus, review)).toMatchObject({ passed: false, reasons: ["final_label_mismatch"] });
  });
});

function corpusFixture(): object {
  return {
    schemaVersion: "dipole.agent.subscription-prefilter-corpus.v1", corpusId: "guardian", revision: "reviewed@1",
    thresholds: { minimumPrecisionBps: 9000, minimumRecallBps: 9000, maximumP95LatencyMicros: 1000, maximumMeanCostMicrousd: 10 },
    cases: [
      { id: "positive", expectedRelevant: true, event: event("positive", "incident text") },
      { id: "negative", expectedRelevant: false, event: event("negative", "hello") }
    ]
  };
}

function reviewFixture(corpusSha256: string) {
  return {
    schemaVersion: "dipole.agent.subscription-corpus-review.v1" as const, corpusSha256, minimumAgreementBps: 5000,
    reviews: [
      { reviewId: "review:a", reviewerId: "reviewer-a", labels: [{ caseId: "positive", relevant: true }, { caseId: "negative", relevant: false }] },
      { reviewId: "review:b", reviewerId: "reviewer-b", labels: [{ caseId: "positive", relevant: false }, { caseId: "negative", relevant: false }] }
    ],
    adjudication: { reviewId: "review:judge", adjudicatorId: "reviewer-c", labels: [{ caseId: "positive", relevant: true }] }
  };
}

function event(id: string, content: string): object {
  return { eventId: `event:${id}`, eventType: "message.direct.created", aggregateId: `message:${id}`, occurredAt: "2026-08-28T00:00:00.000Z", payload: { conversation_key: "group:G1", content } };
}
