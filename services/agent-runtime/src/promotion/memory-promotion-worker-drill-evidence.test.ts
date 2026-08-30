import { describe, expect, it } from "vitest";

import { evaluateMemoryPromotionWorkerDrill } from "./memory-promotion-worker-drill-evidence.js";

const evidence = {
  schemaVersion: "dipole.agent.memory-promotion-worker-drill-evidence.v1" as const,
  sharedEnvironment: true,
  runtimeRevision: "a".repeat(40),
  candidateVersion: "agent-runtime@receipt-worker-v1",
  releaseManifestSha256: "b".repeat(64),
  configurationSha256: "c".repeat(64),
  promotionEvidenceReceiptSha256: "d".repeat(64),
  grantId: "PROMOTION-GRANT-1",
  temporalTaskQueue: "dipole-agent-memory-promotion-v1",
  temporalActivityMode: "promotion_active" as const,
  coreReceiptCommitEnabled: true,
  capabilityRpcMTLS: true,
  operatorAuthority: "operator_approved" as const,
  firstCommit: { receiptId: `MEM-PROMOTE-${"e".repeat(64)}`, receiptSha256: "e".repeat(64), memoryId: "MEM-1", outcome: "committed" as const },
  retry: { receiptId: `MEM-PROMOTE-${"e".repeat(64)}`, receiptSha256: "e".repeat(64), memoryId: "MEM-1", outcome: "replayed" as const },
  revokedGrant: "denied" as const,
  rollback: "rolled_back" as const,
  observedAt: "2026-08-30T12:00:00.000Z"
};

describe("Memory promotion Worker drill evidence", () => {
  it("accepts a shared-environment commit, retry, denial, and rollback drill", () => {
    expect(evaluateMemoryPromotionWorkerDrill(evidence)).toMatchObject({ decision: "eligible", reasons: [] });
  });

  it("blocks drift and incomplete drill outcomes", () => {
    expect(evaluateMemoryPromotionWorkerDrill({ ...evidence, sharedEnvironment: false, rollback: "not_run" })).toMatchObject({
      decision: "blocked", reasons: expect.arrayContaining(["shared_environment_required", "rollback_required"])
    });
    expect(evaluateMemoryPromotionWorkerDrill({ ...evidence, retry: { ...evidence.retry, memoryId: "MEM-2" } })).toMatchObject({
      decision: "blocked", reasons: ["idempotent_retry_required"]
    });
  });
});
