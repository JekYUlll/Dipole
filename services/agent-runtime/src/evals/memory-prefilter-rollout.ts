import {
  evaluateMemoryReviewedCorpus,
  memoryReviewedCorpusReviewSha256,
  parseMemoryReviewedCorpusReview,
  type MemoryReviewedCorpus,
  type MemoryReviewedCorpusReview
} from "./memory-reviewed-corpus.js";
import {
  evaluateMemoryPrefilter,
  type MemoryPrefilterEvidence,
  type MemoryPrefilterPolicy
} from "./memory-prefilter-evaluator.js";

export interface MemoryPrefilterRolloutDecision {
  readonly schemaVersion: "dipole.agent.memory-prefilter-rollout-decision.v1";
  readonly decision: "eligible" | "blocked";
  readonly reasons: Array<"corpus_review_blocked" | "candidate_prefilter_blocked">;
  readonly corpusSha256: string;
  readonly reviewSha256: string;
  readonly finalLabelsSha256: string;
  readonly candidateEvidenceSha256: string;
  readonly candidate: MemoryPrefilterEvidence["candidate"];
  readonly metrics: {
    readonly agreementBps: number;
    readonly minimumAgreementBps: number;
    readonly precisionBps: number;
    readonly recallBps: number;
    readonly p95LatencyMicros: number;
    readonly meanCostMicrousd: number;
  };
}

export function evaluateMemoryPrefilterRollout(
  corpus: MemoryReviewedCorpus,
  rawReview: MemoryReviewedCorpusReview,
  evidence: MemoryPrefilterEvidence,
  policy: MemoryPrefilterPolicy
): MemoryPrefilterRolloutDecision {
  const review = parseMemoryReviewedCorpusReview(rawReview);
  const reviewReport = evaluateMemoryReviewedCorpus(corpus, review);
  const prefilterReport = evaluateMemoryPrefilter(corpus, evidence, policy);
  const reasons: MemoryPrefilterRolloutDecision["reasons"] = [];
  if (!reviewReport.passed) reasons.push("corpus_review_blocked");
  if (!prefilterReport.passed) reasons.push("candidate_prefilter_blocked");
  return {
    schemaVersion: "dipole.agent.memory-prefilter-rollout-decision.v1",
    decision: reasons.length === 0 ? "eligible" : "blocked", reasons,
    corpusSha256: reviewReport.corpusSha256, reviewSha256: memoryReviewedCorpusReviewSha256(review),
    finalLabelsSha256: reviewReport.finalLabelsSha256, candidateEvidenceSha256: prefilterReport.evidenceSha256,
    candidate: prefilterReport.candidate,
    metrics: {
      agreementBps: reviewReport.metrics.agreementBps, minimumAgreementBps: reviewReport.metrics.minimumAgreementBps,
      precisionBps: prefilterReport.metrics.precisionBps, recallBps: prefilterReport.metrics.recallBps,
      p95LatencyMicros: prefilterReport.metrics.p95LatencyMicros, meanCostMicrousd: prefilterReport.metrics.meanCostMicrousd
    }
  };
}
