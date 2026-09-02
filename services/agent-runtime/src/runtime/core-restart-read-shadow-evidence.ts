import { createHash } from "node:crypto";

import { z } from "zod";

import { canonicalMcpJSON } from "../mcp/canonical-json.js";

export const coreRestartReadShadowEvidenceSchemaVersion = "dipole.agent.core-restart-read-shadow.v1" as const;
export const coreRestartReadShadowEvidenceMaximumValidityMs = 24 * 60 * 60 * 1_000;

const eventId = z.string().trim().min(1).max(128);
const outcomeSchema = z.object({
  event_id: eventId,
  core_restart_triggered: z.literal(true),
  core_ready_after_restart: z.literal(true),
  gateway_proxy_recovered: z.literal(true),
  ledger_completed_event_count: z.literal(1),
  task_count: z.literal(1),
  completed_run_count: z.literal(1),
  completed_model_call_count: z.literal(1),
  conversation_digest_artifact_count: z.literal(1)
}).strict();

const payloadSchema = outcomeSchema.extend({
  schema_version: z.literal(coreRestartReadShadowEvidenceSchemaVersion),
  outcome: z.literal("passed"),
  isolation: z.literal("disposable_compose_temporal_kafka_mysql_go_core_read_shadow"),
  collected_at: z.string(),
  expires_at: z.string(),
  production_authority: z.literal(false)
}).strict();

const evidenceSchema = payloadSchema.extend({
  content_sha256: z.string().regex(/^[a-f0-9]{64}$/)
}).strict();

export type CoreRestartReadShadowEvidence = z.infer<typeof evidenceSchema>;
export type CoreRestartReadShadowOutcome = z.infer<typeof outcomeSchema>;

interface EvidenceOptions {
  readonly now?: () => Date;
  readonly validityMs?: number;
}

export function createCoreRestartReadShadowEvidence(
  outcome: CoreRestartReadShadowOutcome,
  options: EvidenceOptions = {}
): CoreRestartReadShadowEvidence {
  const validityMs = options.validityMs ?? coreRestartReadShadowEvidenceMaximumValidityMs;
  if (!Number.isSafeInteger(validityMs) || validityMs < 1 || validityMs > coreRestartReadShadowEvidenceMaximumValidityMs) {
    throw new Error("Core restart read-shadow evidence validity is invalid");
  }
  const now = validDate((options.now ?? (() => new Date()))());
  const payload = payloadSchema.parse({
    schema_version: coreRestartReadShadowEvidenceSchemaVersion,
    outcome: "passed",
    isolation: "disposable_compose_temporal_kafka_mysql_go_core_read_shadow",
    collected_at: now.toISOString(),
    expires_at: new Date(now.getTime() + validityMs).toISOString(),
    ...outcome,
    production_authority: false
  });
  return Object.freeze({ ...payload, content_sha256: digest(payload) });
}

export function parseCoreRestartReadShadowEvidence(
  value: unknown,
  options: Pick<EvidenceOptions, "now"> = {}
): CoreRestartReadShadowEvidence {
  const evidence = evidenceSchema.parse(value);
  const collectedAt = canonicalDate(evidence.collected_at);
  const expiresAt = canonicalDate(evidence.expires_at);
  const now = validDate((options.now ?? (() => new Date()))());
  const { content_sha256: contentSha256, ...payload } = evidence;
  if (expiresAt.getTime() - collectedAt.getTime() < 1 ||
      expiresAt.getTime() - collectedAt.getTime() > coreRestartReadShadowEvidenceMaximumValidityMs ||
      now.getTime() < collectedAt.getTime() || now.getTime() >= expiresAt.getTime() ||
      digest(payload) !== contentSha256) {
    throw new Error("Core restart read-shadow evidence is invalid");
  }
  return Object.freeze(evidence);
}

function digest(payload: z.infer<typeof payloadSchema>): string {
  return createHash("sha256")
    .update(`${coreRestartReadShadowEvidenceSchemaVersion}\n${canonicalMcpJSON(payload)}`, "utf8")
    .digest("hex");
}

function canonicalDate(value: string): Date {
  const parsed = validDate(new Date(value));
  if (parsed.toISOString() !== value) throw new Error("Core restart read-shadow evidence time is invalid");
  return parsed;
}

function validDate(value: Date): Date {
  if (!Number.isFinite(value.getTime())) throw new Error("Core restart read-shadow evidence time is invalid");
  return value;
}
