import { createHash } from "node:crypto";
import { mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { runMemoryPrefilterRolloutCLI } from "./memory-prefilter-rollout-cli.js";
import { canonicalJSON } from "./offline-evaluator.js";

describe("memory prefilter rollout CLI", () => {
  it("rejects missing inputs", async () => {
    const out = sink();
    await expect(runMemoryPrefilterRolloutCLI([], out.stdout, out.stderr)).resolves.toBe(1);
    expect(out.errors).toContain("--corpus=<path>");
  });

  it("emits eligible decision and uses exit code zero", async () => {
    const directory = await mkdtemp(join(tmpdir(), "dipole-memory-prefilter-rollout-"));
    const record = { schemaVersion: "dipole.agent.memory-reviewed-corpus.v1" as const, corpusId: "gold", cases: [
      { caseId: "case-a", candidateType: "message" as const, resourceType: "conversation", evidenceCount: 1, contentSha256: "a".repeat(64), goldPromotable: true },
      { caseId: "case-b", candidateType: "message" as const, resourceType: "conversation", evidenceCount: 1, contentSha256: "b".repeat(64), goldPromotable: false }
    ] };
    const corpus = { ...record, sha256: createHash("sha256").update(canonicalJSON(record)).digest("hex") };
    const review = { schemaVersion: "dipole.agent.memory-reviewed-corpus-review.v1", corpusSha256: corpus.sha256, minimumAgreementBps: 10_000,
      reviews: [
        { reviewId: "review-a", reviewerId: "reviewer-a", labels: [{ caseId: "case-a", promotable: true }, { caseId: "case-b", promotable: false }] },
        { reviewId: "review-b", reviewerId: "reviewer-b", labels: [{ caseId: "case-a", promotable: true }, { caseId: "case-b", promotable: false }] }
      ] };
    const evidence = { schemaVersion: "dipole.agent.memory-prefilter-evidence.v1", corpusSha256: corpus.sha256,
      candidate: { id: "embedding:v1", kind: "embedding", revision: "model@1", configurationSha256: "e".repeat(64), decisionThresholdBps: 7_500 },
      decisions: [{ caseId: "case-a", selected: true, scoreBps: 9_000, latencyMicros: 100, costMicrousd: 1 }, { caseId: "case-b", selected: false, scoreBps: 1_000, latencyMicros: 100, costMicrousd: 1 }] };
    const policy = { minimumPrecisionBps: 10_000, minimumRecallBps: 10_000, maximumP95LatencyMicros: 1_000, maximumMeanCostMicrousd: 5 };
    await Promise.all([
      writeFile(join(directory, "corpus.json"), JSON.stringify(corpus)), writeFile(join(directory, "review.json"), JSON.stringify(review)),
      writeFile(join(directory, "evidence.json"), JSON.stringify(evidence)), writeFile(join(directory, "policy.json"), JSON.stringify(policy))
    ]);
    const out = sink();
    await expect(runMemoryPrefilterRolloutCLI(["--corpus=corpus.json", "--review=review.json", "--evidence=evidence.json", "--policy=policy.json"].map(value => `${value.split("=")[0]}=${join(directory, value.split("=")[1]!)}`), out.stdout, out.stderr)).resolves.toBe(0);
    expect(JSON.parse(out.output)).toMatchObject({ decision: "eligible", reasons: [] });
  });
});

function sink() {
  let output = "";
  let errors = "";
  return {
    stdout: { write: (value: string) => { output += value; } },
    stderr: { write: (value: string) => { errors += value; } },
    get output() { return output; },
    get errors() { return errors; }
  };
}
