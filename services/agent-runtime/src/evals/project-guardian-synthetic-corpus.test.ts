import { readFile } from "node:fs/promises";

import { describe, expect, it } from "vitest";

import { evaluateSubscriptionCorpusReview, parseSubscriptionCorpusReview } from "./subscription-corpus-review.js";
import {
  evaluateSubscriptionPrefilter,
  parseSubscriptionPrefilterCorpus
} from "./subscription-prefilter-evaluator.js";
import { buildRulePrefilterEvidence } from "./subscription-prefilter-rule.js";
import { parseAgentEventSubscription } from "../events/event-subscription.js";

const corpusPath = new URL("../../../../contracts/agent-evals/v1/project-guardian-synthetic-corpus.json", import.meta.url);
const reviewPath = new URL("../../../../contracts/agent-evals/v1/project-guardian-synthetic-review.json", import.meta.url);
const subscriptionPath = new URL("../../../../contracts/agent-evals/v1/project-guardian-synthetic-subscription.json", import.meta.url);

describe("Project Guardian synthetic corpus", () => {
  it("keeps its reviewed trigger baseline reproducible and low-sensitive", async () => {
    const corpus = parseSubscriptionPrefilterCorpus(await readFile(corpusPath, "utf8"));
    const review = parseSubscriptionCorpusReview(await readFile(reviewPath, "utf8"));

    expect(corpus).toMatchObject({
      corpusId: "project-guardian-synthetic",
      revision: "project-guardian-synthetic@v1",
      cases: expect.any(Array)
    });
    expect(corpus.cases).toHaveLength(8);
    expect(corpus.cases.filter(item => item.expectedRelevant)).toHaveLength(4);
    expect(corpus.cases.every(item => item.event.eventId.startsWith("fixture:")
      && item.event.aggregateId.startsWith("fixture-")
      && item.event.payload.content.startsWith("FIXTURE:"))).toBe(true);
    expect(evaluateSubscriptionCorpusReview(corpus, review)).toMatchObject({
      passed: true,
      reasons: [],
      metrics: { totalCases: 8, agreementBps: 10_000, adjudicatedCases: 0 }
    });
  });

  it("runs the production deterministic matcher through the common evaluator", async () => {
    const corpus = parseSubscriptionPrefilterCorpus(await readFile(corpusPath, "utf8"));
    const subscription = parseAgentEventSubscription(JSON.parse(await readFile(subscriptionPath, "utf8")));
    let nowMicros = 0;
    const evidence = buildRulePrefilterEvidence(corpus, subscription, {
      nowMicros: () => (nowMicros += 20)
    });

    expect(evaluateSubscriptionPrefilter(corpus, evidence)).toMatchObject({
      passed: true,
      reasons: [],
      confusion: { truePositive: 4, trueNegative: 4, falsePositive: 0, falseNegative: 0 },
      metrics: { precisionBps: 10_000, recallBps: 10_000, p95LatencyMicros: 20, meanCostMicrousd: 0 }
    });
  });
});
