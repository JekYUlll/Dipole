import { chmod, mkdtemp, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it, afterEach } from "vitest";

import { canonicalJSON } from "./offline-evaluator.js";
import { createHash } from "node:crypto";
import {
  createMemoryReviewedCorpusSourceManifest,
  loadMemoryReviewedCorpusSource,
  loadMemoryReviewedCorpusSourceManifest
} from "./memory-reviewed-corpus-source.js";

const directories: string[] = [];
afterEach(async () => Promise.all(directories.splice(0).map(path => rm(path, { recursive: true, force: true }))));

describe("Memory reviewed corpus source", () => {
  it("loads an owner-only corpus and review under an unexpired approval manifest", async () => {
    const directory = await mkdtemp(join(tmpdir(), "dipole-memory-corpus-source-"));
    directories.push(directory);
    const corpus = { schemaVersion: "dipole.agent.memory-reviewed-corpus.v1", corpusId: "memory-corpus:source", cases: [{ caseId: "case-1", candidateType: "message", resourceType: "conversation", evidenceCount: 1, contentSha256: "a".repeat(64), goldPromotable: true }] };
    const corpusSha256 = createHash("sha256").update(canonicalJSON(corpus)).digest("hex");
    const review = { schemaVersion: "dipole.agent.memory-reviewed-corpus-review.v1", corpusSha256, minimumAgreementBps: 10_000, reviews: [{ reviewId: "review:one", reviewerId: "reviewer:one", labels: [{ caseId: "case-1", promotable: true }] }, { reviewId: "review:two", reviewerId: "reviewer:two", labels: [{ caseId: "case-1", promotable: true }] }] };
    const corpusPath = join(directory, "corpus.json");
    const reviewPath = join(directory, "review.json");
    await writeFile(corpusPath, JSON.stringify({ ...corpus, sha256: corpusSha256 }), { mode: 0o600 });
    await writeFile(reviewPath, JSON.stringify(review), { mode: 0o600 });
    const reviewSha256 = createHash("sha256").update(canonicalJSON(review)).digest("hex");
    const manifest = { schemaVersion: "dipole.agent.memory-reviewed-corpus-source.v1", sourceId: "source:example", ownerUid: process.getuid?.() ?? 0, corpusPath, reviewPath, corpusSha256, reviewSha256, approvedAt: "2026-08-29T01:00:00.000Z", expiresAt: "2026-08-29T02:00:00.000Z" } as const;
    const manifestPath = join(directory, "source-manifest.json");
    await writeFile(manifestPath, JSON.stringify(manifest), { mode: 0o600 });
    await expect(loadMemoryReviewedCorpusSourceManifest(manifestPath)).resolves.toMatchObject({ sourceId: "source:example" });
    const loaded = await loadMemoryReviewedCorpusSource(manifest, new Date("2026-08-29T01:30:00.000Z"));
    expect(loaded.corpus.corpusId).toBe("memory-corpus:source");
  });

  it("rejects symlinked, broadly readable, expired, and hash-drifted sources", async () => {
    const directory = await mkdtemp(join(tmpdir(), "dipole-memory-corpus-source-"));
    directories.push(directory);
    const corpusPath = join(directory, "corpus.json");
    const reviewPath = join(directory, "review.json");
    await writeFile(corpusPath, "{}", { mode: 0o644 });
    await writeFile(reviewPath, "{}", { mode: 0o600 });
    const manifest = { schemaVersion: "dipole.agent.memory-reviewed-corpus-source.v1", sourceId: "source:example", ownerUid: process.getuid?.() ?? 0, corpusPath, reviewPath, corpusSha256: "a".repeat(64), reviewSha256: "b".repeat(64), approvedAt: "2026-08-29T01:00:00.000Z", expiresAt: "2026-08-29T02:00:00.000Z" };
    await expect(loadMemoryReviewedCorpusSource(manifest, new Date("2026-08-29T01:30:00.000Z"))).rejects.toThrow(/permissions|invalid/i);
    await chmod(corpusPath, 0o600);
    await expect(loadMemoryReviewedCorpusSource({ ...manifest, expiresAt: "2026-08-29T01:10:00.000Z" }, new Date("2026-08-29T01:30:00.000Z"))).rejects.toThrow(/expired|window/i);
    const link = join(directory, "link.json");
    await symlink(corpusPath, link);
    await expect(loadMemoryReviewedCorpusSource({ ...manifest, corpusPath: link }, new Date("2026-08-29T01:30:00.000Z"))).rejects.toThrow(/source|symlink|ELOOP/i);
  });

  it("builds an owner-bound manifest from secure source files without accepting caller hashes", async () => {
    const directory = await mkdtemp(join(tmpdir(), "dipole-memory-corpus-source-"));
    directories.push(directory);
    const corpus = { schemaVersion: "dipole.agent.memory-reviewed-corpus.v1", corpusId: "memory-corpus:manifest", cases: [{ caseId: "case-1", candidateType: "message", resourceType: "conversation", evidenceCount: 1, contentSha256: "c".repeat(64), goldPromotable: true }] };
    const corpusSha256 = createHash("sha256").update(canonicalJSON(corpus)).digest("hex");
    const review = { schemaVersion: "dipole.agent.memory-reviewed-corpus-review.v1", corpusSha256, minimumAgreementBps: 10_000, reviews: [{ reviewId: "review:one", reviewerId: "reviewer:one", labels: [{ caseId: "case-1", promotable: true }] }, { reviewId: "review:two", reviewerId: "reviewer:two", labels: [{ caseId: "case-1", promotable: true }] }] };
    const corpusPath = join(directory, "corpus.json");
    const reviewPath = join(directory, "review.json");
    await writeFile(corpusPath, JSON.stringify({ ...corpus, sha256: corpusSha256 }), { mode: 0o600 });
    await writeFile(reviewPath, JSON.stringify(review), { mode: 0o600 });

    const manifest = await createMemoryReviewedCorpusSourceManifest({
      sourceId: "source:manifest", corpusPath, reviewPath,
      approvedAt: "2026-08-29T01:00:00.000Z", expiresAt: "2026-08-29T02:00:00.000Z"
    });

    expect(manifest).toMatchObject({ ownerUid: process.getuid?.(), corpusSha256, sourceId: "source:manifest" });
    expect(await loadMemoryReviewedCorpusSource(manifest, new Date("2026-08-29T01:30:00.000Z"))).toMatchObject({ corpus: { corpusId: "memory-corpus:manifest" } });
  });
});
