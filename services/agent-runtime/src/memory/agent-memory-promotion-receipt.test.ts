import { describe, expect, it } from "vitest";

import {
  createAgentMemoryPromotionReceipt,
  replayAgentMemoryPromotionReceipt,
  validateAgentMemoryPromotionReceipt,
  type AgentMemoryPromotionIntent
} from "./agent-memory-promotion-receipt.js";

const now = new Date("2026-08-29T01:00:00.000Z");

function intent(overrides: Partial<AgentMemoryPromotionIntent> = {}): AgentMemoryPromotionIntent {
  return {
    tenantId: "dipole", principalUserId: "U100", agentId: "UAI", taskId: "TASK-1", runId: "RUN-1",
    candidateId: "CAND-1", candidateSha256: "a".repeat(64), reviewId: "REV-1", policyVersion: "memory-v1",
    expiresAt: new Date(now.getTime() + 10 * 60 * 1000).toISOString(), ...overrides
  };
}

describe("Agent Memory promotion receipt", () => {
  it("creates a deterministic low-sensitivity receipt", () => {
    const first = createAgentMemoryPromotionReceipt(intent(), now);
    const second = createAgentMemoryPromotionReceipt(intent(), now);

    expect(first).toEqual(second);
    expect(first).toMatchObject({
      schemaVersion: "dipole.agent.memory-promotion-receipt.v1", status: "prepared",
      tenantId: "dipole", taskId: "TASK-1", runId: "RUN-1", candidateId: "CAND-1", reviewId: "REV-1"
    });
    expect(first.receiptId).toMatch(/^MEM-PROMOTE-[a-f0-9]{64}$/);
    expect(first.receiptSha256).toMatch(/^[a-f0-9]{64}$/);
    expect(JSON.stringify(first)).not.toMatch(/summary|evidence|secret|token/i);
    expect(validateAgentMemoryPromotionReceipt(first)).toEqual(first);
  });

  it("replays only the exact intent and rejects drift or expiry", () => {
    const receipt = createAgentMemoryPromotionReceipt(intent(), now);

    expect(replayAgentMemoryPromotionReceipt(receipt, intent(), new Date("2026-08-29T01:05:00.000Z"))).toEqual(receipt);
    expect(() => replayAgentMemoryPromotionReceipt(receipt, intent({ reviewId: "REV-2" }), now)).toThrow(/conflict/i);
    expect(() => replayAgentMemoryPromotionReceipt(receipt, intent(), new Date("2026-08-29T01:11:00.000Z"))).toThrow(/expired/i);
  });

  it("fails closed on receipt hash, time, and binding tampering", () => {
    const receipt = createAgentMemoryPromotionReceipt(intent(), now);
    expect(() => validateAgentMemoryPromotionReceipt({ ...receipt, receiptSha256: "f".repeat(64) })).toThrow(/hash/i);
    expect(() => validateAgentMemoryPromotionReceipt({ ...receipt, status: "committed" })).toThrow(/hash/i);
    expect(() => validateAgentMemoryPromotionReceipt({ ...receipt, expiresAt: now.toISOString() })).toThrow(/hash|time|expiry/i);
    expect(() => validateAgentMemoryPromotionReceipt({ ...receipt, candidateSha256: "b".repeat(64) })).toThrow(/hash/i);
  });
});
