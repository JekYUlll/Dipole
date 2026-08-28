import { describe, expect, it, vi } from "vitest";

import { ObservationWorker } from "./observation-worker.js";
import {
  MySQLMemoryCandidateReviewLedger,
  buildMemoryCandidateReview,
  type MemoryCandidateReviewConnection,
} from "./memory-candidate-review.js";

const candidate = new ObservationWorker().observe({
  tenantId: "dipole", principalId: "U100", agentId: "UAI", resourceType: "conversation", resourceId: "group:G1",
  eventId: "EV-1", messageId: "M-1", messageSequence: "42", senderId: "U200",
  occurredAt: "2026-08-29T00:00:00.000Z", content: "决定：周五前完成 API v2。",
})[0]!;

function review() {
  return buildMemoryCandidateReview(candidate, "a".repeat(64), "U100", "accepted", "owner confirmed", "2026-08-29T00:01:00.000Z");
}

function connection(): MemoryCandidateReviewConnection & { calls: string[] } {
  const calls: string[] = [];
  return {
    calls,
    beginTransaction: vi.fn(async () => { calls.push("begin"); }),
    commit: vi.fn(async () => { calls.push("commit"); }),
    rollback: vi.fn(async () => { calls.push("rollback"); }),
    release: vi.fn(),
    execute: vi.fn(async (sql: string): Promise<[unknown, unknown]> => {
      calls.push(sql);
      if (sql.startsWith("INSERT")) return [{ affectedRows: 1 }, []];
      if (sql.startsWith("UPDATE")) return [{ affectedRows: 1 }, []];
      return [[], []];
    }),
  };
}

describe("memory candidate review contract", () => {
  it("binds a decision to the exact candidate hash and never carries full content", () => {
    const value = review();
    expect(value).toMatchObject({ candidateId: candidate.memoryId, decision: "accepted", reviewerId: "U100" });
    expect(value).not.toHaveProperty("content");
    expect(value.reviewId).toMatch(/^REVIEW-[a-f0-9]{64}$/);
    expect(() => buildMemoryCandidateReview(candidate, "b".repeat(64), "U100", "accepted", "owner confirmed", value.reviewedAt)).not.toThrow();
  });

  it("rejects unbounded or credential-shaped review reasons", () => {
    expect(() => buildMemoryCandidateReview(candidate, "a".repeat(64), "U100", "accepted", "token=secret", review().reviewedAt)).toThrow(/reason/i);
    expect(() => buildMemoryCandidateReview(candidate, "bad", "U100", "accepted", "ok", review().reviewedAt)).toThrow(/hash/i);
  });
});

describe("MySQLMemoryCandidateReviewLedger", () => {
  it("updates candidate status and review audit in one transaction", async () => {
    const db = connection();
    await expect(new MySQLMemoryCandidateReviewLedger(() => db).append(review())).resolves.toEqual({ outcome: "inserted" });
    expect(db.calls[0]).toBe("begin");
    expect(db.calls).toContain("UPDATE agent_memory_candidates SET status = ? WHERE candidate_uuid = ? AND candidate_sha256 = ? AND status = 'pending'");
    expect(db.calls.at(-1)).toBe("commit");
  });
});
