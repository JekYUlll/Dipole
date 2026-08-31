import { createHash } from "node:crypto";

import { z } from "zod";

import { canonicalMcpJSON } from "../mcp/canonical-json.js";

export const approvalGateDrillEvidenceSchemaVersion = "dipole.agent.approval-gate-drill.v1" as const;
export const approvalGateDrillEvidenceMaximumValidityMs = 24 * 60 * 60 * 1_000;

const outcomeSchema = z.object({
  approved_effect_count: z.literal(1),
  denied_effect_count: z.literal(0),
  consumed_replay_effect_count: z.literal(0),
  failed_effect_count: z.literal(1),
  failed_replay_effect_count: z.literal(0),
  core_rpc_type: z.literal("go_internal_grpc_mtls"),
  core_rpc_authenticated: z.literal(true)
}).strict();

const payloadSchema = outcomeSchema.extend({
  schema_version: z.literal(approvalGateDrillEvidenceSchemaVersion),
  outcome: z.literal("passed"),
  isolation: z.literal("disposable_go_core_mtls_approval_gate_fixture"),
  collected_at: z.string(),
  expires_at: z.string(),
  production_authority: z.literal(false)
}).strict();

const evidenceSchema = payloadSchema.extend({
  content_sha256: z.string().regex(/^[a-f0-9]{64}$/)
}).strict();

export type ApprovalGateDrillEvidence = z.infer<typeof evidenceSchema>;
export type ApprovalGateDrillEvidenceOutcome = z.infer<typeof outcomeSchema>;

export function createApprovalGateDrillEvidence(
  outcome: ApprovalGateDrillEvidenceOutcome,
  options: { readonly now?: () => Date; readonly validityMs?: number } = {}
): ApprovalGateDrillEvidence {
  const validityMs = options.validityMs ?? approvalGateDrillEvidenceMaximumValidityMs;
  if (!Number.isSafeInteger(validityMs) || validityMs < 1 || validityMs > approvalGateDrillEvidenceMaximumValidityMs) {
    throw new Error("Approval gate drill evidence validity is invalid");
  }
  const collectedAt = validDate((options.now ?? (() => new Date()))());
  const payload = payloadSchema.parse({
    schema_version: approvalGateDrillEvidenceSchemaVersion,
    outcome: "passed",
    isolation: "disposable_go_core_mtls_approval_gate_fixture",
    collected_at: collectedAt.toISOString(),
    expires_at: new Date(collectedAt.getTime() + validityMs).toISOString(),
    ...outcomeSchema.parse(outcome),
    production_authority: false
  });
  return Object.freeze({ ...payload, content_sha256: digest(payload) });
}

export function parseApprovalGateDrillEvidence(value: unknown, options: { readonly now?: () => Date } = {}): ApprovalGateDrillEvidence {
  const evidence = evidenceSchema.parse(value);
  const collectedAt = canonicalDate(evidence.collected_at);
  const expiresAt = canonicalDate(evidence.expires_at);
  const now = validDate((options.now ?? (() => new Date()))());
  const { content_sha256: contentSha256, ...payload } = evidence;
  if (expiresAt.getTime() - collectedAt.getTime() < 1 || expiresAt.getTime() - collectedAt.getTime() > approvalGateDrillEvidenceMaximumValidityMs ||
      now.getTime() < collectedAt.getTime() || now.getTime() >= expiresAt.getTime() || digest(payload) !== contentSha256) {
    throw new Error("Approval gate drill evidence is invalid");
  }
  return Object.freeze(evidence);
}

function digest(payload: z.infer<typeof payloadSchema>): string {
  return createHash("sha256").update(`${approvalGateDrillEvidenceSchemaVersion}\n${canonicalMcpJSON(payload)}`, "utf8").digest("hex");
}

function canonicalDate(value: string): Date {
  const parsed = validDate(new Date(value));
  if (parsed.toISOString() !== value) throw new Error("Approval gate drill evidence time is invalid");
  return parsed;
}

function validDate(value: Date): Date {
  if (!Number.isFinite(value.getTime())) throw new Error("Approval gate drill evidence time is invalid");
  return value;
}
