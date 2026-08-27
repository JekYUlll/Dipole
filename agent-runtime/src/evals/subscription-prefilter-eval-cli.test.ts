import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterEach, describe, expect, it } from "vitest";

import { runSubscriptionPrefilterEvalCLI } from "./subscription-prefilter-eval-cli.js";

const temporaryDirectories: string[] = [];
afterEach(async () => Promise.all(temporaryDirectories.splice(0).map(path => rm(path, { recursive: true, force: true }))));

describe("subscription prefilter eval CLI", () => {
  it("generates a rule baseline without echoing event content", async () => {
    const directory = await mkdtemp(join(tmpdir(), "dipole-prefilter-"));
    temporaryDirectories.push(directory);
    const corpusPath = join(directory, "corpus.json");
    const subscriptionPath = join(directory, "subscription.json");
    await writeFile(corpusPath, JSON.stringify(corpus()), "utf8");
    await writeFile(subscriptionPath, JSON.stringify(subscription()), "utf8");
    const output: string[] = [];
    const errors: string[] = [];

    const code = await runSubscriptionPrefilterEvalCLI(
      [`--corpus=${corpusPath}`, `--subscription=${subscriptionPath}`],
      { write: value => output.push(value) }, { write: value => errors.push(value) }
    );
    expect(code).toBe(0);
    expect(errors).toEqual([]);
    expect(JSON.parse(output.join(""))).toMatchObject({
      evidence: { candidate: { kind: "rule" } },
      report: { passed: true, metrics: { precisionBps: 10_000, recallBps: 10_000, meanCostMicrousd: 0 } }
    });
    expect(output.join("")).not.toContain("Incident detected");

    await writeFile(subscriptionPath, JSON.stringify({ ...subscription(), filter: { terms: ["never-matches"] } }), "utf8");
    const blockedOutput: string[] = [];
    const blockedCode = await runSubscriptionPrefilterEvalCLI(
      [`--corpus=${corpusPath}`, `--subscription=${subscriptionPath}`],
      { write: value => blockedOutput.push(value) }, { write: value => errors.push(value) }
    );
    expect(blockedCode).toBe(2);
    expect(JSON.parse(blockedOutput.join(""))).toMatchObject({
      report: { passed: false, reasons: ["precision_below_minimum", "recall_below_minimum"] }
    });
  });

  it("uses exit 1 for invalid arguments", async () => {
    const errors: string[] = [];
    const code = await runSubscriptionPrefilterEvalCLI([], { write: () => undefined }, { write: value => errors.push(value) });
    expect(code).toBe(1);
    expect(errors.join("")).toMatch(/exactly one/iu);
  });
});

function corpus(): object {
  return {
    schemaVersion: "dipole.agent.subscription-prefilter-corpus.v1", corpusId: "guardian-events", revision: "reviewed@1",
    thresholds: { minimumPrecisionBps: 10_000, minimumRecallBps: 10_000, maximumP95LatencyMicros: 1_000_000, maximumMeanCostMicrousd: 0 },
    cases: [
      { id: "relevant", expectedRelevant: true, event: event("relevant", "Incident detected") },
      { id: "irrelevant", expectedRelevant: false, event: event("irrelevant", "weekly hello") }
    ]
  };
}

function event(id: string, content: string): object {
  return {
    eventId: `event:${id}`, eventType: "message.direct.created", aggregateId: `message:${id}`,
    occurredAt: "2026-08-28T00:00:00.000Z", payload: { conversation_key: "group:G1", content }
  };
}

function subscription(): object {
  return {
    subscriptionId: "SUB-RULE", definitionId: "DEF-1", definitionVersion: 1, tenantId: "dipole", agentId: "UAI",
    eventType: "message.direct.created", resourceType: "conversation", resourceId: "group:G1",
    filterKind: "message_contains_any", filter: { terms: ["incident"] }
  };
}
