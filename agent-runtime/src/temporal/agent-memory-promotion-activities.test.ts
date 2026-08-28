import { describe, expect, it } from "vitest";

import { foundationAgentTaskActivities } from "./agent-task-activities.js";

describe("Temporal Agent Memory promotion receipt Activity", () => {
  it("returns a replayable receipt without carrying candidate content", async () => {
    const receipt = await foundationAgentTaskActivities.prepareAgentMemoryPromotion({
      tenantId: "dipole", principalUserId: "U100", agentId: "UAI", taskId: "TASK-1", runId: "RUN-1",
      candidateId: "CAND-1", candidateSha256: "a".repeat(64), reviewId: "REV-1", policyVersion: "memory-v1",
      createdAt: "2026-08-29T01:00:00.000Z", expiresAt: "2026-08-29T01:10:00.000Z"
    });
    expect(receipt.status).toBe("prepared");
    expect(receipt.receiptId).toMatch(/^MEM-PROMOTE-[a-f0-9]{64}$/);
    expect(JSON.stringify(receipt)).not.toMatch(/summary|evidence|secret|token/i);
  });

  it("rejects a receipt window longer than the bounded preparation lease", async () => {
    await expect(foundationAgentTaskActivities.prepareAgentMemoryPromotion({
      tenantId: "dipole", principalUserId: "U100", agentId: "UAI", taskId: "TASK-1", runId: "RUN-1",
      candidateId: "CAND-1", candidateSha256: "a".repeat(64), reviewId: "REV-1", policyVersion: "memory-v1",
      createdAt: "2026-08-29T01:00:00.000Z", expiresAt: "2026-08-29T02:00:00.000Z"
    })).rejects.toThrow(/expiry/i);
  });
});
