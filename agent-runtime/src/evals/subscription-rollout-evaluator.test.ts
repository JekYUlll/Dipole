import { describe, expect, it } from "vitest";

import { parseSubscriptionCorpusReview } from "./subscription-corpus-review.js";
import { parseSubscriptionPrefilterCorpus, parseSubscriptionPrefilterEvidence } from "./subscription-prefilter-evaluator.js";
import { evaluateSubscriptionRollout } from "./subscription-rollout-evaluator.js";

describe("subscription rollout evaluator", () => {
  it("recomputes review and candidate evidence before returning eligible", () => {
    const sources = fixtures();
    const decision = evaluateSubscriptionRollout(sources.corpus, sources.review, sources.evidence);
    expect(decision).toMatchObject({
      decision: "eligible", reasons: [], candidate: { kind: "embedding" },
      metrics: { agreementBps: 10_000, precisionBps: 10_000, recallBps: 10_000, meanCostMicrousd: 1 }
    });
    expect(decision.corpusSha256).toBe(sources.corpus.sha256);
    expect(JSON.stringify(decision)).not.toContain("incident text");
    expect(JSON.stringify(decision)).not.toContain("reviewer-a");
  });

  it("blocks independently on review and candidate thresholds", () => {
    const sources = fixtures();
    const strictReview = parseSubscriptionCorpusReview({ ...sources.review, minimumAgreementBps: 10_000,
      reviews: [sources.review.reviews[0], { ...sources.review.reviews[1], labels: [{ caseId: "negative", relevant: false }, { caseId: "positive", relevant: false }] }],
      adjudication: { reviewId: "review:judge", adjudicatorId: "reviewer-c", labels: [{ caseId: "positive", relevant: true }] }
    });
    expect(evaluateSubscriptionRollout(sources.corpus, strictReview, sources.evidence)).toMatchObject({ decision: "blocked", reasons: ["corpus_review_blocked"] });

    const missed = parseSubscriptionPrefilterEvidence({ ...sources.evidence, decisions: sources.evidence.decisions.map(item => item.caseId === "positive" ? { ...item, selected: false, scoreBps: 1000 } : item) });
    expect(evaluateSubscriptionRollout(sources.corpus, sources.review, missed)).toMatchObject({ decision: "blocked", reasons: ["candidate_prefilter_blocked"] });
  });

  it("rejects source evidence bound to another corpus", () => {
    const sources = fixtures();
    const drifted = parseSubscriptionPrefilterEvidence({ ...sources.evidence, corpusSha256: "f".repeat(64) });
    expect(() => evaluateSubscriptionRollout(sources.corpus, sources.review, drifted)).toThrow(/corpus/iu);
  });
});

function fixtures() {
  const corpus = parseSubscriptionPrefilterCorpus({
    schemaVersion: "dipole.agent.subscription-prefilter-corpus.v1", corpusId: "guardian", revision: "reviewed@1",
    thresholds: { minimumPrecisionBps: 9000, minimumRecallBps: 9000, maximumP95LatencyMicros: 1000, maximumMeanCostMicrousd: 2 },
    cases: [
      { id: "positive", expectedRelevant: true, event: event("positive", "incident text") },
      { id: "negative", expectedRelevant: false, event: event("negative", "hello") }
    ]
  });
  const review = parseSubscriptionCorpusReview({
    schemaVersion: "dipole.agent.subscription-corpus-review.v1", corpusSha256: corpus.sha256, minimumAgreementBps: 10_000,
    reviews: [
      { reviewId: "review:a", reviewerId: "reviewer-a", labels: [{ caseId: "positive", relevant: true }, { caseId: "negative", relevant: false }] },
      { reviewId: "review:b", reviewerId: "reviewer-b", labels: [{ caseId: "positive", relevant: true }, { caseId: "negative", relevant: false }] }
    ]
  });
  const evidence = parseSubscriptionPrefilterEvidence({
    schemaVersion: "dipole.agent.subscription-prefilter-evidence.v1", corpusSha256: corpus.sha256,
    candidate: { id: "embedding:v1", kind: "embedding", revision: "embed@1", configurationSha256: "a".repeat(64), decisionThresholdBps: 5000 },
    decisions: [
      { caseId: "positive", selected: true, scoreBps: 9000, latencyMicros: 100, costMicrousd: 1 },
      { caseId: "negative", selected: false, scoreBps: 1000, latencyMicros: 100, costMicrousd: 1 }
    ]
  });
  return { corpus, review, evidence };
}

function event(id: string, content: string): object {
  return { eventId: `event:${id}`, eventType: "message.direct.created", aggregateId: `message:${id}`, occurredAt: "2026-08-28T00:00:00.000Z", payload: { conversation_key: "group:G1", content } };
}
