import { createHash } from "node:crypto";

import { z } from "zod";

import { canonicalMcpJSON } from "../mcp/canonical-json.js";

export const externalMcpShadowDrillEvidenceSchemaVersion =
  "dipole.agent.external-mcp-shadow-drill.v2" as const;
export const externalMcpShadowDrillEvidenceMaximumValidityMs = 24 * 60 * 60 * 1_000;

const outcomeSchema = z.object({
  event_count: z.literal(2),
  ledger_completed_event_count: z.literal(2),
  tool_call_count: z.literal(1),
  artifact_count: z.literal(1),
  restart_duplicate_suppressed: z.literal(true),
  expired_readiness_denied: z.literal(true),
  core_rpc_type: z.literal("go_internal_grpc_mtls"),
  core_rpc_authenticated: z.literal(true),
  core_rpc_identity_denials_verified: z.literal(true)
}).strict();

const payloadSchema = outcomeSchema.extend({
  schema_version: z.literal(externalMcpShadowDrillEvidenceSchemaVersion),
  outcome: z.literal("passed"),
  isolation: z.literal("disposable_mysql_kafka_temporal_go_core_mtls_and_local_mcp"),
  collected_at: z.string(),
  expires_at: z.string(),
  production_authority: z.literal(false)
}).strict();

const evidenceSchema = payloadSchema.extend({
  content_sha256: z.string().regex(/^[a-f0-9]{64}$/)
}).strict();

export type ExternalMcpShadowDrillEvidence = z.infer<typeof evidenceSchema>;

export type ExternalMcpShadowDrillEvidenceOutcome = z.infer<typeof outcomeSchema>;

interface EvidenceClockOptions {
  readonly now?: () => Date;
}

interface EvidenceCreationOptions extends EvidenceClockOptions {
  readonly validityMs?: number;
}

export function createExternalMcpShadowDrillEvidence(
  outcome: ExternalMcpShadowDrillEvidenceOutcome,
  options: EvidenceCreationOptions = {}
): ExternalMcpShadowDrillEvidence {
  const validityMs = options.validityMs ?? externalMcpShadowDrillEvidenceMaximumValidityMs;
  if (!Number.isSafeInteger(validityMs) || validityMs < 1 || validityMs > externalMcpShadowDrillEvidenceMaximumValidityMs) {
    throw new Error("External MCP Shadow drill evidence validity is invalid");
  }
  const validatedOutcome = outcomeSchema.parse(outcome);
  const collectedAt = validDate((options.now ?? (() => new Date()))());
  const payload = payloadSchema.parse({
    schema_version: externalMcpShadowDrillEvidenceSchemaVersion,
    outcome: "passed",
    isolation: "disposable_mysql_kafka_temporal_go_core_mtls_and_local_mcp",
    collected_at: collectedAt.toISOString(),
    expires_at: new Date(collectedAt.getTime() + validityMs).toISOString(),
    ...validatedOutcome,
    production_authority: false
  });
  return Object.freeze({ ...payload, content_sha256: digest(payload) });
}

export function parseExternalMcpShadowDrillEvidence(
  value: unknown,
  options: EvidenceClockOptions = {}
): ExternalMcpShadowDrillEvidence {
  const evidence = evidenceSchema.parse(value);
  const collectedAt = canonicalDate(evidence.collected_at);
  const expiresAt = canonicalDate(evidence.expires_at);
  const now = validDate((options.now ?? (() => new Date()))());
  const validityMs = expiresAt.getTime() - collectedAt.getTime();
  const { content_sha256: contentSha256, ...payload } = evidence;
  if (validityMs < 1 || validityMs > externalMcpShadowDrillEvidenceMaximumValidityMs ||
      now.getTime() < collectedAt.getTime() || now.getTime() >= expiresAt.getTime() ||
      digest(payload) !== contentSha256) {
    throw new Error("External MCP Shadow drill evidence is invalid");
  }
  return Object.freeze(evidence);
}

function digest(payload: z.infer<typeof payloadSchema>): string {
  return createHash("sha256")
    .update(`${externalMcpShadowDrillEvidenceSchemaVersion}\n${canonicalMcpJSON(payload)}`, "utf8")
    .digest("hex");
}

function canonicalDate(value: string): Date {
  const parsed = validDate(new Date(value));
  if (parsed.toISOString() !== value) throw new Error("External MCP Shadow drill evidence time is invalid");
  return parsed;
}

function validDate(value: Date): Date {
  if (!Number.isFinite(value.getTime())) throw new Error("External MCP Shadow drill evidence time is invalid");
  return value;
}
