import { createHash } from "node:crypto";

import { z } from "zod";

const sha256 = z.string().regex(/^[a-f0-9]{64}$/);
const commit = z.object({
  receiptId: z.string().regex(/^MEM-PROMOTE-[a-f0-9]{64}$/),
  receiptSha256: sha256,
  memoryId: z.string().trim().min(1).max(128),
  outcome: z.enum(["committed", "replayed", "failed", "not_run"])
}).strict();

export const memoryPromotionWorkerDrillEvidenceSchema = z.object({
  schemaVersion: z.literal("dipole.agent.memory-promotion-worker-drill-evidence.v1"),
  sharedEnvironment: z.boolean(),
  runtimeRevision: z.string().regex(/^(?:[a-f0-9]{40}|[a-f0-9]{64})$/),
  candidateVersion: z.string().trim().min(2).max(128),
  releaseManifestSha256: sha256,
  configurationSha256: sha256,
  promotionEvidenceReceiptSha256: sha256,
  grantId: z.string().trim().min(1).max(128),
  temporalTaskQueue: z.string().trim().min(1).max(256),
  temporalActivityMode: z.enum(["promotion_active", "other"]),
  coreReceiptCommitEnabled: z.boolean(),
  capabilityRpcMTLS: z.boolean(),
  operatorAuthority: z.enum(["operator_approved", "other"]),
  firstCommit: commit,
  retry: commit,
  revokedGrant: z.enum(["denied", "failed", "not_run"]),
  rollback: z.enum(["rolled_back", "failed", "not_run"]),
  observedAt: z.iso.datetime()
}).strict();

export type MemoryPromotionWorkerDrillEvidence = z.infer<typeof memoryPromotionWorkerDrillEvidenceSchema>;

export interface MemoryPromotionWorkerDrillDecision {
  readonly schemaVersion: "dipole.agent.memory-promotion-worker-drill-decision.v1";
  readonly decision: "eligible" | "blocked";
  readonly reasons: readonly string[];
  readonly evidenceSha256: string;
}

export function evaluateMemoryPromotionWorkerDrill(raw: unknown): MemoryPromotionWorkerDrillDecision {
  const evidence = memoryPromotionWorkerDrillEvidenceSchema.parse(raw);
  const reasons: string[] = [];
  if (!evidence.sharedEnvironment) reasons.push("shared_environment_required");
  if (evidence.temporalActivityMode !== "promotion_active") reasons.push("promotion_active_required");
  if (!evidence.coreReceiptCommitEnabled) reasons.push("core_receipt_commit_required");
  if (!evidence.capabilityRpcMTLS) reasons.push("capability_rpc_mtls_required");
  if (evidence.operatorAuthority !== "operator_approved") reasons.push("operator_authority_required");
  if (evidence.firstCommit.outcome !== "committed") reasons.push("first_commit_required");
  if (evidence.retry.outcome !== "replayed" || evidence.retry.receiptId !== evidence.firstCommit.receiptId || evidence.retry.receiptSha256 !== evidence.firstCommit.receiptSha256 || evidence.retry.memoryId !== evidence.firstCommit.memoryId) {
    reasons.push("idempotent_retry_required");
  }
  if (evidence.revokedGrant !== "denied") reasons.push("revoked_grant_denial_required");
  if (evidence.rollback !== "rolled_back") reasons.push("rollback_required");
  return {
    schemaVersion: "dipole.agent.memory-promotion-worker-drill-decision.v1",
    decision: reasons.length === 0 ? "eligible" : "blocked",
    reasons,
    evidenceSha256: digest(evidence)
  };
}

function digest(value: MemoryPromotionWorkerDrillEvidence): string {
  return createHash("sha256").update(canonicalJSON(value), "utf8").digest("hex");
}

function canonicalJSON(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(canonicalJSON).join(",")}]`;
  if (value !== null && typeof value === "object") {
    return `{${Object.entries(value).sort(([left], [right]) => left.localeCompare(right)).map(([key, item]) => `${JSON.stringify(key)}:${canonicalJSON(item)}`).join(",")}}`;
  }
  return JSON.stringify(value);
}
