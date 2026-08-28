import { evaluateSubscriptionCorpusReview, type SubscriptionCorpusReview } from "./subscription-corpus-review.js";
import {
  evaluateSubscriptionPrefilter,
  type SubscriptionPrefilterCorpus,
  type SubscriptionPrefilterEvidence
} from "./subscription-prefilter-evaluator.js";

export interface SubscriptionRolloutDecision {
  readonly schemaVersion: "dipole.agent.subscription-rollout-decision.v1";
  readonly decision: "eligible" | "blocked";
  readonly reasons: Array<"corpus_review_blocked" | "candidate_prefilter_blocked">;
  readonly corpusSha256: string;
  readonly reviewSha256: string;
  readonly finalLabelsSha256: string;
  readonly candidateEvidenceSha256: string;
  readonly candidate: SubscriptionPrefilterEvidence["candidate"];
  readonly metrics: {
    readonly agreementBps: number;
    readonly minimumAgreementBps: number;
    readonly precisionBps: number;
    readonly recallBps: number;
    readonly p95LatencyMicros: number;
    readonly meanCostMicrousd: number;
  };
}

export function evaluateSubscriptionRollout(
  corpus: SubscriptionPrefilterCorpus,
  review: SubscriptionCorpusReview,
  evidence: SubscriptionPrefilterEvidence
): SubscriptionRolloutDecision {
  const reviewReport = evaluateSubscriptionCorpusReview(corpus, review);
  const prefilterReport = evaluateSubscriptionPrefilter(corpus, evidence);
  const reasons: SubscriptionRolloutDecision["reasons"] = [];
  if (!reviewReport.passed) reasons.push("corpus_review_blocked");
  if (!prefilterReport.passed) reasons.push("candidate_prefilter_blocked");
  return {
    schemaVersion: "dipole.agent.subscription-rollout-decision.v1",
    decision: reasons.length === 0 ? "eligible" : "blocked", reasons,
    corpusSha256: reviewReport.corpusSha256, reviewSha256: reviewReport.reviewSha256,
    finalLabelsSha256: reviewReport.finalLabelsSha256, candidateEvidenceSha256: prefilterReport.evidenceSha256,
    candidate: prefilterReport.candidate,
    metrics: {
      agreementBps: reviewReport.metrics.agreementBps, minimumAgreementBps: reviewReport.metrics.minimumAgreementBps,
      precisionBps: prefilterReport.metrics.precisionBps, recallBps: prefilterReport.metrics.recallBps,
      p95LatencyMicros: prefilterReport.metrics.p95LatencyMicros, meanCostMicrousd: prefilterReport.metrics.meanCostMicrousd
    }
  };
}
