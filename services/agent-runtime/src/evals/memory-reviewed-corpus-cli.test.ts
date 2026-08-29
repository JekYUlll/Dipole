import { mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { runMemoryReviewedCorpusCLI } from "./memory-reviewed-corpus-cli.js";
import { canonicalJSON } from "./offline-evaluator.js";
import { createHash } from "node:crypto";

describe("memory reviewed corpus CLI", () => {
  it("returns a low-sensitivity passing gate result", async () => {
    const directory = await mkdtemp(join(tmpdir(), "dipole-memory-corpus-"));
    const corpus = {
      schemaVersion: "dipole.agent.memory-reviewed-corpus.v1", corpusId: "memory-corpus:cli",
      cases: [{ caseId: "case-1", candidateType: "message", resourceType: "conversation", evidenceCount: 1, contentSha256: "a".repeat(64), goldPromotable: true }]
    };
    const corpusSha256 = createHash("sha256").update(canonicalJSON(corpus)).digest("hex");
    const review = {
      schemaVersion: "dipole.agent.memory-reviewed-corpus-review.v1", corpusSha256, minimumAgreementBps: 10_000,
      reviews: [
        { reviewId: "review:one", reviewerId: "reviewer:one", labels: [{ caseId: "case-1", promotable: true }] },
        { reviewId: "review:two", reviewerId: "reviewer:two", labels: [{ caseId: "case-1", promotable: true }] }
      ]
    };
    const corpusPath = join(directory, "corpus.json");
    const reviewPath = join(directory, "review.json");
    await writeFile(corpusPath, JSON.stringify({ ...corpus, sha256: corpusSha256 }));
    await writeFile(reviewPath, JSON.stringify(review));
    const output: string[] = [];
    const errors: string[] = [];
    const code = await runMemoryReviewedCorpusCLI([`--corpus=${corpusPath}`, `--review=${reviewPath}`], { write: value => { output.push(String(value)); return true; } }, { write: value => { errors.push(String(value)); return true; } });
    expect(code).toBe(0);
    expect(JSON.parse(output.join("")).metrics).toMatchObject({ totalCases: 1, promotableCases: 1 });
    expect(output.join("")).not.toContain("reviewer:one");
    expect(errors).toEqual([]);
  });

  it("returns usage error without files", async () => {
    const errors: string[] = [];
    await expect(runMemoryReviewedCorpusCLI([], { write: () => true }, { write: value => { errors.push(String(value)); return true; } })).resolves.toBe(1);
    expect(errors.join("")).toMatch(/requires/);
  });
});
