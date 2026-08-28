import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { afterEach, describe, expect, it } from "vitest";

import { parseSubscriptionPrefilterCorpus } from "./subscription-prefilter-evaluator.js";
import { runSubscriptionCorpusReviewCLI } from "./subscription-corpus-review-cli.js";

const directories: string[] = [];
afterEach(async () => Promise.all(directories.splice(0).map(path => rm(path, { recursive: true, force: true }))));

describe("subscription corpus review CLI", () => {
  it("returns low-sensitive reports and distinguishes failed evidence", async () => {
    const directory = await mkdtemp(join(tmpdir(), "dipole-corpus-review-"));
    directories.push(directory);
    const corpusPath = join(directory, "corpus.json");
    const reviewPath = join(directory, "review.json");
    const corpus = corpusFixture();
    const hash = parseSubscriptionPrefilterCorpus(corpus).sha256;
    await writeFile(corpusPath, JSON.stringify(corpus), "utf8");
    await writeFile(reviewPath, JSON.stringify(reviewFixture(hash, 10_000)), "utf8");
    const output: string[] = [];
    const errors: string[] = [];
    expect(await runSubscriptionCorpusReviewCLI([`--corpus=${corpusPath}`, `--review=${reviewPath}`], { write: value => output.push(value) }, { write: value => errors.push(value) })).toBe(2);
    expect(JSON.parse(output.join(""))).toMatchObject({ passed: false, reasons: ["agreement_below_minimum"] });
    expect(output.join("")).not.toContain("Incident detected");
    expect(output.join("")).not.toContain("reviewer-a");
    expect(errors).toEqual([]);
  });

  it("returns exit 1 for invalid arguments", async () => {
    const errors: string[] = [];
    expect(await runSubscriptionCorpusReviewCLI([], { write: () => undefined }, { write: value => errors.push(value) })).toBe(1);
    expect(errors.join("")).toMatch(/requires/iu);
  });
});

function corpusFixture(): object {
  return {
    schemaVersion: "dipole.agent.subscription-prefilter-corpus.v1", corpusId: "guardian", revision: "reviewed@1",
    thresholds: { minimumPrecisionBps: 9000, minimumRecallBps: 9000, maximumP95LatencyMicros: 1000, maximumMeanCostMicrousd: 10 },
    cases: [
      { id: "positive", expectedRelevant: true, event: event("positive", "Incident detected") },
      { id: "negative", expectedRelevant: false, event: event("negative", "Hello") }
    ]
  };
}

function reviewFixture(corpusSha256: string, minimumAgreementBps: number): object {
  return {
    schemaVersion: "dipole.agent.subscription-corpus-review.v1", corpusSha256, minimumAgreementBps,
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
