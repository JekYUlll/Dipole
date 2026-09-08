import { createHash } from "node:crypto";

import { describe, expect, it } from "vitest";

import { validateContextAblationReviewedSource } from "./context-ablation-reviewed-source.js";
import type { MemoryReviewedCorpus, MemoryReviewedCorpusReview } from "./memory-reviewed-corpus.js";
import type { LoadedMemoryReviewedCorpus } from "./memory-reviewed-corpus-source.js";
import { canonicalJSON } from "./offline-evaluator.js";

const hash = (value: string) => value.repeat(64).slice(0, 64);

function source(): Pick<LoadedMemoryReviewedCorpus, "manifest" | "corpus" | "review"> {
  const cases = [
    { caseId: "case:one", candidateType: "message" as const, resourceType: "conversation", evidenceCount: 2, contentSha256: hash("a"), goldPromotable: true },
    { caseId: "case:two", candidateType: "reflection" as const, resourceType: "conversation", evidenceCount: 3, contentSha256: hash("b"), goldPromotable: false }
  ];
  const corpus: MemoryReviewedCorpus = { schemaVersion: "dipole.agent.memory-reviewed-corpus.v1", corpusId: "corpus:reviewed", cases, sha256: "" };
  corpus.sha256 = createHash("sha256").update(canonicalJSON({ schemaVersion: corpus.schemaVersion, corpusId: corpus.corpusId, cases })).digest("hex");
  const review: MemoryReviewedCorpusReview = {
    schemaVersion: "dipole.agent.memory-reviewed-corpus-review.v1",
    corpusSha256: corpus.sha256,
    minimumAgreementBps: 10_000,
    reviews: [
      { reviewId: "review:one", reviewerId: "reviewer:one", labels: [{ caseId: "case:one", promotable: true }, { caseId: "case:two", promotable: false }] },
      { reviewId: "review:two", reviewerId: "reviewer:two", labels: [{ caseId: "case:one", promotable: true }, { caseId: "case:two", promotable: false }] }
    ]
  };
  return {
    manifest: {
      schemaVersion: "dipole.agent.memory-reviewed-corpus-source.v1" as const,
      sourceId: "source:reviewed", ownerUid: 0, corpusPath: "/secure/corpus.json", reviewPath: "/secure/review.json",
      corpusSha256: corpus.sha256, reviewSha256: createHash("sha256").update(canonicalJSON(review)).digest("hex"),
      approvedAt: "2026-09-09T00:00:00.000Z", expiresAt: "2026-09-10T00:00:00.000Z"
    },
    corpus,
    review
  };
}

function manifest(cases = [hash("a"), hash("b")]) {
  return {
    schemaVersion: "dipole.agent.context-ablation-manifest.v1",
    experimentId: "experiment:reviewed", candidateVersion: "agent@reviewed",
    routePrices: [{ route: "example/flash", inputMicrousdPerMillionTokens: 1, outputMicrousdPerMillionTokens: 1 }],
    cases: cases.map(caseSha256 => ({ caseSha256, requiredOutputIds: ["artifact:summary:v1"], relevantEvidenceIds: ["evidence:reviewed"] }))
  };
}

describe("Context ablation reviewed source", () => {
  it("requires complete owner-reviewed corpus coverage and emits a path-free receipt", () => {
    const receipt = validateContextAblationReviewedSource(manifest(), source());
    expect(receipt).toMatchObject({ sourceId: "source:reviewed", corpusSha256: source().corpus.sha256 });
    expect(JSON.stringify(receipt)).not.toMatch(/secure|case:|reviewer/i);
  });

  it("fails closed when experiment coverage or review gates drift", () => {
    expect(() => validateContextAblationReviewedSource(manifest([hash("a")]), source())).toThrow(/cover exactly/i);
    const rejected = source();
    rejected.review.reviews[1]!.labels[0] = { caseId: "case:one", promotable: false };
    expect(() => validateContextAblationReviewedSource(manifest(), rejected)).toThrow(/adjudication/i);
  });
});
