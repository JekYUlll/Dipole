import { describe, expect, it, vi } from "vitest";

import { createAgentMemoryPromotionReceipt } from "../memory/agent-memory-promotion-receipt.js";
import { createAgentMemoryPromotionCommitActivities } from "./agent-memory-promotion-commit-activity.js";
import { foundationAgentTaskActivities } from "./agent-task-activities.js";

const receipt = createAgentMemoryPromotionReceipt({
  tenantId: "dipole", principalUserId: "U100", agentId: "UAI", taskId: "TASK-1", runId: "RUN-1",
  candidateId: "CAND-1", candidateSha256: "a".repeat(64), reviewId: "REV-1", policyVersion: "memory-v1",
  candidateMemoryType: "observational", targetMemoryType: "semantic", expiresAt: "2026-08-30T02:10:00.000Z"
}, new Date("2026-08-30T02:00:00.000Z"));

describe("Temporal Agent Memory promotion commit Activity", () => {
  it("forwards only the prepared receipt and correlation to the active RPC client", async () => {
    const commitMemoryPromotionReceipt = vi.fn(async () => ({
      memoryId: "MEM-1", memoryType: "semantic" as const, status: "active" as const, receiptSha256: receipt.receiptSha256,
      provenance: { sourceType: "memory_candidate" as const, sourceId: receipt.candidateId, sequence: receipt.reviewId }
    }));
    const activities = createAgentMemoryPromotionCommitActivities({ commitMemoryPromotionReceipt });

    await expect(activities.commitPreparedAgentMemoryPromotion({ receipt, requestId: "REQ-1", traceId: "TRACE-1" }))
      .resolves.toMatchObject({ memoryId: "MEM-1", status: "active" });
    expect(commitMemoryPromotionReceipt).toHaveBeenCalledWith(receipt, { requestId: "REQ-1", traceId: "TRACE-1" });
  });

  it("keeps the foundation Worker fail-closed", async () => {
    await expect(foundationAgentTaskActivities.commitPreparedAgentMemoryPromotion({ receipt }))
      .rejects.toThrow(/not enabled/);
  });
});
