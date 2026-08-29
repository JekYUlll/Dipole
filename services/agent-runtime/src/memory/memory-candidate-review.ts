import { createHash } from "node:crypto";

import type { ResultSetHeader } from "mysql2/promise";
import { z } from "zod";

import { parseMemoryCandidate, type MemoryCandidate } from "./observation-worker.js";

const identifier = z.string().trim().min(1).max(128).regex(/^[A-Za-z0-9][A-Za-z0-9_.:/-]*$/);
const hash = z.string().regex(/^[a-f0-9]{64}$/);
const reviewSchema = z.object({
  schemaVersion: z.literal("dipole.agent.memory-candidate-review.v1"),
  reviewId: z.string().regex(/^REVIEW-[a-f0-9]{64}$/),
  candidateId: identifier,
  candidateSha256: hash,
  reviewerId: identifier,
  decision: z.enum(["accepted", "rejected"]),
  reason: z.string().trim().min(1).max(1000),
  reviewSha256: hash,
  reviewedAt: z.iso.datetime(),
}).strict();

const INSERT_REVIEW = `INSERT INTO agent_memory_candidate_reviews (
  review_uuid, candidate_uuid, candidate_sha256, reviewer_uuid, decision, reason, review_sha256, reviewed_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`;
const UPDATE_CANDIDATE = "UPDATE agent_memory_candidates SET status = ? WHERE candidate_uuid = ? AND candidate_sha256 = ? AND status = 'pending'";
const GET_REVIEW = "SELECT review_sha256 FROM agent_memory_candidate_reviews WHERE candidate_uuid = ? LIMIT 1";

export type MemoryCandidateReview = z.infer<typeof reviewSchema>;

export interface MemoryCandidateReviewConnection {
  beginTransaction(): Promise<void>;
  commit(): Promise<void>;
  rollback(): Promise<void>;
  release(): void;
  execute(sql: string, values?: unknown[]): Promise<[unknown, unknown]>;
}

export function parseMemoryCandidateReview(value: unknown): MemoryCandidateReview {
  return reviewSchema.parse(value);
}

export function buildMemoryCandidateReview(
  rawCandidate: MemoryCandidate,
  candidateSha256: string,
  reviewerId: string,
  decision: "accepted" | "rejected",
  reason: string,
  reviewedAt: string,
): MemoryCandidateReview {
  const candidate = parseMemoryCandidate(rawCandidate);
  if (typeof candidateSha256 !== "string" || !/^[a-f0-9]{64}$/.test(candidateSha256)) throw new Error("Agent Memory candidate hash is invalid");
  const candidateHash = candidateSha256;
  const reviewer = identifier.parse(reviewerId);
  const normalizedReason = reason.trim();
  if (!normalizedReason || normalizedReason.length > 1000 || /(?:password|passwd|token|secret|authorization|bearer|api[_ -]?key)\s*[:=]/i.test(normalizedReason)) {
    throw new Error("Agent Memory candidate review reason is invalid");
  }
  const time = z.iso.datetime().parse(reviewedAt);
  const canonical = JSON.stringify({
    schemaVersion: "dipole.agent.memory-candidate-review.v1",
    candidateId: candidate.memoryId, candidateSha256: candidateHash, reviewerId: reviewer,
    decision, reason: normalizedReason, reviewedAt: time,
  });
  const reviewSha256 = createHash("sha256").update(canonical, "utf8").digest("hex");
  return parseMemoryCandidateReview({
    schemaVersion: "dipole.agent.memory-candidate-review.v1",
    reviewId: `REVIEW-${reviewSha256}`,
    candidateId: candidate.memoryId,
    candidateSha256: candidateHash,
    reviewerId: reviewer,
    decision,
    reason: normalizedReason,
    reviewSha256,
    reviewedAt: time,
  });
}

export type CandidateReviewAppendResult = { readonly outcome: "inserted" | "duplicate" };

export class MySQLMemoryCandidateReviewLedger {
  constructor(private readonly connectionFactory: () => MemoryCandidateReviewConnection | Promise<MemoryCandidateReviewConnection>) {}

  async append(review: MemoryCandidateReview): Promise<CandidateReviewAppendResult> {
    const value = parseMemoryCandidateReview(review);
    const connection = await this.connectionFactory();
    await connection.beginTransaction();
    try {
      try {
        const [result] = await connection.execute(INSERT_REVIEW, [
          value.reviewId, value.candidateId, value.candidateSha256, value.reviewerId,
          value.decision, value.reason, value.reviewSha256, value.reviewedAt,
        ]) as [ResultSetHeader, unknown];
        if (result.affectedRows !== 1) throw new Error("Agent Memory candidate review was not recorded");
      } catch (error) {
        if (!isDuplicateKey(error)) throw error;
        const [rows] = await connection.execute(GET_REVIEW, [value.candidateId]) as [Array<{ review_sha256: string }>, unknown];
        const existing = rows[0];
        if (existing === undefined) throw new Error("Agent Memory candidate review write is indeterminate");
        if (existing.review_sha256 !== value.reviewSha256) throw new Error(`Agent Memory candidate review conflict for ${value.candidateId}`);
        await connection.commit();
        return { outcome: "duplicate" };
      }

      const [result] = await connection.execute(UPDATE_CANDIDATE, [value.decision, value.candidateId, value.candidateSha256]) as [ResultSetHeader, unknown];
      if (result.affectedRows !== 1) throw new Error(`Agent Memory candidate ${value.candidateId} is missing or already reviewed`);
      await connection.commit();
      return { outcome: "inserted" };
    } catch (error) {
      await connection.rollback().catch(() => undefined);
      throw error;
    } finally {
      connection.release();
    }
  }
}

function isDuplicateKey(error: unknown): boolean {
  return typeof error === "object" && error !== null && "code" in error && error.code === "ER_DUP_ENTRY";
}
