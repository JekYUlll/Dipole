import { constants } from "node:fs";
import { open, realpath } from "node:fs/promises";
import { isAbsolute, dirname, resolve } from "node:path";

import { z } from "zod";

import {
  memoryReviewedCorpusReviewSha256,
  parseMemoryReviewedCorpus,
  parseMemoryReviewedCorpusReview,
  type MemoryReviewedCorpus,
  type MemoryReviewedCorpusReview
} from "./memory-reviewed-corpus.js";

const sourceId = z.string().trim().min(2).max(128).regex(/^[a-z0-9][a-z0-9._:-]*$/u);
const sha256 = z.string().regex(/^[a-f0-9]{64}$/u);
const manifestSchema = z.object({
  schemaVersion: z.literal("dipole.agent.memory-reviewed-corpus-source.v1"), sourceId,
  ownerUid: z.number().int().nonnegative(), corpusPath: z.string().min(1), reviewPath: z.string().min(1),
  corpusSha256: sha256, reviewSha256: sha256, approvedAt: z.string().datetime({ offset: true }), expiresAt: z.string().datetime({ offset: true })
}).strict();
const manifestInputSchema = manifestSchema.omit({ ownerUid: true, corpusSha256: true, reviewSha256: true, schemaVersion: true });

export type MemoryReviewedCorpusSourceManifest = z.infer<typeof manifestSchema>;

export interface LoadedMemoryReviewedCorpus {
  readonly manifest: MemoryReviewedCorpusSourceManifest;
  readonly corpus: MemoryReviewedCorpus;
  readonly review: MemoryReviewedCorpusReview;
}

/** Reads an owner-only source manifest before following its referenced files. */
export async function loadMemoryReviewedCorpusSourceManifest(path: string): Promise<MemoryReviewedCorpusSourceManifest> {
  return manifestSchema.parse(JSON.parse(await secureRead(await securePath(path))) as unknown);
}

export async function createMemoryReviewedCorpusSourceManifest(
  rawInput: unknown,
  currentUid: number | undefined = typeof process.getuid === "function" ? process.getuid() : undefined
): Promise<MemoryReviewedCorpusSourceManifest> {
  const input = manifestInputSchema.parse(rawInput);
  if (currentUid === undefined) throw new Error("memory corpus source owner is unavailable");
  const approvedAt = parseTimestamp(input.approvedAt, "approval");
  const expiresAt = parseTimestamp(input.expiresAt, "expiry");
  if (expiresAt <= approvedAt) throw new Error("memory corpus source approval window is invalid");
  const { corpus, review } = await readMemoryReviewedCorpusFiles(input.corpusPath, input.reviewPath);
  if (review.corpusSha256 !== corpus.sha256) throw new Error("memory corpus review SHA-256 does not match the corpus");
  return manifestSchema.parse({
    schemaVersion: "dipole.agent.memory-reviewed-corpus-source.v1",
    ...input,
    ownerUid: currentUid,
    corpusSha256: corpus.sha256,
    reviewSha256: memoryReviewedCorpusReviewSha256(review)
  });
}

export async function loadMemoryReviewedCorpusSource(
  rawManifest: unknown,
  now: Date = new Date(),
  currentUid: number | undefined = typeof process.getuid === "function" ? process.getuid() : undefined
): Promise<LoadedMemoryReviewedCorpus> {
  const manifest = manifestSchema.parse(rawManifest);
  const currentTime = validClock(now);
  const approvedAt = parseTimestamp(manifest.approvedAt, "approval");
  const expiresAt = parseTimestamp(manifest.expiresAt, "expiry");
  if (expiresAt <= approvedAt || currentTime < approvedAt || currentTime >= expiresAt) throw new Error("memory corpus source approval window is invalid or expired");
  if (currentUid !== undefined && manifest.ownerUid !== currentUid) throw new Error("memory corpus source owner does not match the current process");
  const { corpus, review } = await readMemoryReviewedCorpusFiles(manifest.corpusPath, manifest.reviewPath);
  if (corpus.sha256 !== manifest.corpusSha256 || memoryReviewedCorpusReviewSha256(review) !== manifest.reviewSha256) {
    throw new Error("memory corpus source hash does not match its approval manifest");
  }
  return { manifest, corpus, review };
}

async function readMemoryReviewedCorpusFiles(corpusPath: string, reviewPath: string): Promise<{ corpus: MemoryReviewedCorpus; review: MemoryReviewedCorpusReview }> {
  const [resolvedCorpusPath, resolvedReviewPath] = await Promise.all([securePath(corpusPath), securePath(reviewPath)]);
  const [corpusRaw, reviewRaw] = await Promise.all([secureRead(resolvedCorpusPath), secureRead(resolvedReviewPath)]);
  return { corpus: parseMemoryReviewedCorpus(corpusRaw), review: parseMemoryReviewedCorpusReview(reviewRaw) };
}

async function securePath(rawPath: string): Promise<string> {
  if (!isAbsolute(rawPath)) throw new Error("memory corpus source paths must be absolute");
  const path = resolve(rawPath);
  if (path !== rawPath) throw new Error("memory corpus source path must be canonical");
  const parent = dirname(path);
  if (await realpath(parent) !== parent) throw new Error("memory corpus source parent must be canonical");
  return path;
}

async function secureRead(path: string): Promise<string> {
  const handle = await open(path, constants.O_RDONLY | (constants.O_NOFOLLOW ?? 0));
  try {
    const stat = await handle.stat();
    if (!stat.isFile() || stat.nlink !== 1 || stat.size > 2 * 1024 * 1024 || (stat.mode & 0o077) !== 0) throw new Error("memory corpus source file permissions or size are invalid");
    return await handle.readFile("utf8");
  } finally {
    await handle.close();
  }
}

function parseTimestamp(value: string, name: string): number {
  const parsed = Date.parse(value);
  if (!Number.isFinite(parsed) || new Date(parsed).toISOString() !== new Date(value).toISOString()) throw new Error(`memory corpus source ${name} timestamp is invalid`);
  return parsed;
}

function validClock(value: Date): number {
  const parsed = value.getTime();
  if (!Number.isFinite(parsed)) throw new Error("memory corpus source clock is invalid");
  return parsed;
}
