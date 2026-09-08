import { mkdtemp, readFile, stat, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createHash } from "node:crypto";
import { describe, expect, it } from "vitest";

import { runMemoryReviewedCorpusManifestCLI } from "./memory-reviewed-corpus-manifest-cli.js";
import { canonicalJSON } from "./offline-evaluator.js";

describe("memory reviewed corpus manifest CLI", () => {
  it("writes a new owner-bound manifest and emits no source paths", async () => {
    const directory = await mkdtemp(join(tmpdir(), "dipole-memory-corpus-manifest-"));
    const corpus = { schemaVersion: "dipole.agent.memory-reviewed-corpus.v1", corpusId: "memory-corpus:manifest-cli", cases: [{ caseId: "case-1", candidateType: "message", resourceType: "conversation", evidenceCount: 1, contentSha256: "d".repeat(64), goldPromotable: true }] };
    const corpusSha256 = createHash("sha256").update(canonicalJSON(corpus)).digest("hex");
    const review = { schemaVersion: "dipole.agent.memory-reviewed-corpus-review.v1", corpusSha256, minimumAgreementBps: 10_000, reviews: [{ reviewId: "review:one", reviewerId: "reviewer:one", labels: [{ caseId: "case-1", promotable: true }] }, { reviewId: "review:two", reviewerId: "reviewer:two", labels: [{ caseId: "case-1", promotable: true }] }] };
    const corpusPath = join(directory, "corpus.json");
    const reviewPath = join(directory, "review.json");
    const outputPath = join(directory, "manifest.json");
    await writeFile(corpusPath, JSON.stringify({ ...corpus, sha256: corpusSha256 }), { mode: 0o600 });
    await writeFile(reviewPath, JSON.stringify(review), { mode: 0o600 });
    const output: string[] = [];
    const errors: string[] = [];

    const code = await runMemoryReviewedCorpusManifestCLI([
      "--source-id=source:manifest-cli", `--corpus=${corpusPath}`, `--review=${reviewPath}`,
      "--approved-at=2026-08-29T01:00:00.000Z", "--expires-at=2026-08-29T02:00:00.000Z", `--output=${outputPath}`
    ], { write: value => { output.push(String(value)); return true; } }, { write: value => { errors.push(String(value)); return true; } });

    expect(code).toBe(0);
    expect(JSON.parse(output.join(""))).toMatchObject({ sourceId: "source:manifest-cli", corpusSha256 });
    expect(output.join("")).not.toContain(directory);
    expect(JSON.parse(await readFile(outputPath, "utf8"))).toMatchObject({ sourceId: "source:manifest-cli", corpusPath, reviewPath, corpusSha256 });
    expect((await stat(outputPath)).mode & 0o777).toBe(0o600);
    expect(errors).toEqual([]);
  });

  it("returns usage error when required arguments are absent", async () => {
    const errors: string[] = [];
    await expect(runMemoryReviewedCorpusManifestCLI([], { write: () => true }, { write: value => { errors.push(String(value)); return true; } })).resolves.toBe(1);
    expect(errors.join("")).toMatch(/requires/);
  });
});
