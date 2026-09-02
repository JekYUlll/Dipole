import { readFile } from "node:fs/promises";

import { describe, expect, it } from "vitest";

import {
  evaluateSubscriptionPrefilter,
  parseSubscriptionPrefilterCorpus,
  parseSubscriptionPrefilterEvidence
} from "./subscription-prefilter-evaluator.js";
import { buildRulePrefilterEvidence } from "./subscription-prefilter-rule.js";

describe("subscription prefilter evaluator", () => {
  it("keeps the language-neutral example corpus hash stable", async () => {
    const source = await readFile(new URL("../../../../contracts/agent-subscription-prefilter/v1/corpus.example.json", import.meta.url), "utf8");
    expect(parseSubscriptionPrefilterCorpus(source).sha256).toBe("e3a0eb42bb6f40c6a81ff233fb468bb996452287d05b9b52b2908941605d3ab2");
  });

  it("produces hash-bound classification, latency, and cost evidence", () => {
    const corpus = parseSubscriptionPrefilterCorpus(corpusFixture());
    const evidence = parseSubscriptionPrefilterEvidence({
      schemaVersion: "dipole.agent.subscription-prefilter-evidence.v1",
      corpusSha256: corpus.sha256,
      candidate: {
        id: "small-model:v1", kind: "small_model", revision: "model@2026-08-28",
        configurationSha256: "a".repeat(64), decisionThresholdBps: 7000
      },
      decisions: [
        decision("relevant", true, 9000, 100, 3),
        decision("irrelevant", false, 1000, 200, 1),
        decision("missed", false, 6500, 300, 2),
        decision("false-positive", true, 8000, 400, 2)
      ]
    });

    const report = evaluateSubscriptionPrefilter(corpus, evidence);
    expect(report).toMatchObject({
      schemaVersion: "dipole.agent.subscription-prefilter-report.v1",
      passed: false,
      confusion: { truePositive: 1, trueNegative: 1, falsePositive: 1, falseNegative: 1 },
      metrics: { precisionBps: 5000, recallBps: 5000, p95LatencyMicros: 400, meanCostMicrousd: 2 },
      falsePositiveCaseIds: ["false-positive"], falseNegativeCaseIds: ["missed"]
    });
    expect(report.reasons).toEqual(["precision_below_minimum", "recall_below_minimum"]);
    expect(report.corpusSha256).toBe(corpus.sha256);
    expect(report.evidenceSha256).toMatch(/^[a-f0-9]{64}$/u);
  });

  it("rejects incomplete, duplicated, drifted, or score-inconsistent evidence", () => {
    const corpus = parseSubscriptionPrefilterCorpus(corpusFixture());
    const base = {
      schemaVersion: "dipole.agent.subscription-prefilter-evidence.v1",
      corpusSha256: corpus.sha256,
      candidate: {
        id: "embedding:v1", kind: "embedding", revision: "embed@1",
        configurationSha256: "b".repeat(64), decisionThresholdBps: 5000
      },
      decisions: corpus.cases.map(item => decision(item.id, item.expectedRelevant, item.expectedRelevant ? 9000 : 1000, 10, 0))
    };
    expect(() => parseSubscriptionPrefilterEvidence({ ...base, decisions: base.decisions.slice(1) })).not.toThrow();
    expect(() => evaluateSubscriptionPrefilter(corpus, parseSubscriptionPrefilterEvidence({ ...base, decisions: base.decisions.slice(1) }))).toThrow(/exactly/iu);
    expect(() => parseSubscriptionPrefilterEvidence({ ...base, decisions: [base.decisions[0], ...base.decisions] })).toThrow(/unique/iu);
    expect(() => parseSubscriptionPrefilterEvidence({ ...base, corpusSha256: "c".repeat(64) })).not.toThrow();
    expect(() => evaluateSubscriptionPrefilter(corpus, parseSubscriptionPrefilterEvidence({ ...base, corpusSha256: "c".repeat(64) }))).toThrow(/corpus/iu);
    expect(() => parseSubscriptionPrefilterEvidence({ ...base, decisions: [{ ...base.decisions[0], selected: !base.decisions[0]!.selected }, ...base.decisions.slice(1)] })).toThrow(/threshold/iu);
  });

  it("uses the production rule matcher for the zero-cost baseline", () => {
    const corpus = parseSubscriptionPrefilterCorpus(corpusFixture());
    const evidence = buildRulePrefilterEvidence(corpus, {
      subscriptionId: "SUB-RULE", definitionId: "DEF-1", definitionVersion: 1,
      tenantId: "dipole", agentId: "UAI", eventType: "message.direct.created",
      resourceType: "conversation", resourceId: "group:G1",
      filterKind: "message_contains_any", filter: { terms: ["INCIDENT", "延期"] }
    }, { nowMicros: monotonicMicros() });
    expect(evidence.decisions.map(item => [item.caseId, item.selected, item.costMicrousd])).toEqual([
      ["false-positive", false, 0], ["irrelevant", false, 0], ["missed", false, 0], ["relevant", true, 0]
    ]);
    expect(evidence.candidate).toMatchObject({ kind: "rule" });
    expect(evidence.candidate.id).toMatch(/^rule:[a-f0-9]{64}$/u);
    expect(evaluateSubscriptionPrefilter(corpus, evidence)).toMatchObject({ passed: false, reasons: ["recall_below_minimum"] });
  });
});

function corpusFixture(): object {
  return {
    schemaVersion: "dipole.agent.subscription-prefilter-corpus.v1",
    corpusId: "guardian-events", revision: "reviewed@2026-08-28",
    thresholds: { minimumPrecisionBps: 7500, minimumRecallBps: 7500, maximumP95LatencyMicros: 1000, maximumMeanCostMicrousd: 5 },
    cases: [
      corpusCase("relevant", true, "Incident detected", "group:G1"),
      corpusCase("irrelevant", false, "weekly hello", "group:G1"),
      corpusCase("missed", true, "schedule risk", "group:G1"),
      corpusCase("false-positive", false, "incident elsewhere", "group:G2")
    ]
  };
}

function corpusCase(id: string, expectedRelevant: boolean, content: string, conversationKey: string): object {
  return {
    id, expectedRelevant,
    event: {
      eventId: `event:${id}`, eventType: "message.direct.created", aggregateId: `message:${id}`,
      occurredAt: "2026-08-28T00:00:00.000Z", payload: { conversation_key: conversationKey, content }
    }
  };
}

function decision(caseId: string, selected: boolean, scoreBps: number, latencyMicros: number, costMicrousd: number): { caseId: string; selected: boolean; scoreBps: number; latencyMicros: number; costMicrousd: number } {
  return { caseId, selected, scoreBps, latencyMicros, costMicrousd };
}

function monotonicMicros(): () => number {
  let value = 0;
  return () => ++value;
}
