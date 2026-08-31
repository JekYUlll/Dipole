import { createHash } from "node:crypto";

import { z } from "zod";

import { canonicalMcpJSON } from "../mcp/canonical-json.js";

export const eventLeaseReclaimEvidenceSchemaVersion = "dipole.agent.event-lease-reclaim.v1" as const;
export const eventLeaseReclaimEvidenceMaximumValidityMs = 24 * 60 * 60 * 1_000;

const identity = z.string().trim().min(1).max(128);
const outcomeSchema = z.object({
  event_id: identity,
  task_id: identity,
  expired_claim_reclaimed: z.literal(true),
  stale_owner_completion_rejected: z.literal(true),
  reclaim_attempt_count: z.literal(2),
  completed_event_count: z.literal(1)
}).strict();

const payloadSchema = outcomeSchema.extend({
  schema_version: z.literal(eventLeaseReclaimEvidenceSchemaVersion),
  outcome: z.literal("passed"),
  isolation: z.literal("disposable_mysql_agent_event_ledger_lease_reclaim"),
  collected_at: z.string(),
  expires_at: z.string(),
  production_authority: z.literal(false)
}).strict();

const evidenceSchema = payloadSchema.extend({ content_sha256: z.string().regex(/^[a-f0-9]{64}$/) }).strict();

export type EventLeaseReclaimEvidence = z.infer<typeof evidenceSchema>;
export type EventLeaseReclaimOutcome = z.infer<typeof outcomeSchema>;

interface EvidenceOptions {
  readonly now?: () => Date;
  readonly validityMs?: number;
}

export function createEventLeaseReclaimEvidence(
  outcome: EventLeaseReclaimOutcome,
  options: EvidenceOptions = {}
): EventLeaseReclaimEvidence {
  const validityMs = options.validityMs ?? eventLeaseReclaimEvidenceMaximumValidityMs;
  if (!Number.isSafeInteger(validityMs) || validityMs < 1 || validityMs > eventLeaseReclaimEvidenceMaximumValidityMs) {
    throw new Error("Event lease reclaim evidence validity is invalid");
  }
  const now = validDate((options.now ?? (() => new Date()))());
  const payload = payloadSchema.parse({
    schema_version: eventLeaseReclaimEvidenceSchemaVersion,
    outcome: "passed",
    isolation: "disposable_mysql_agent_event_ledger_lease_reclaim",
    collected_at: now.toISOString(),
    expires_at: new Date(now.getTime() + validityMs).toISOString(),
    ...outcome,
    production_authority: false
  });
  return Object.freeze({ ...payload, content_sha256: digest(payload) });
}

export function parseEventLeaseReclaimEvidence(
  value: unknown,
  options: Pick<EvidenceOptions, "now"> = {}
): EventLeaseReclaimEvidence {
  const evidence = evidenceSchema.parse(value);
  const collectedAt = canonicalDate(evidence.collected_at);
  const expiresAt = canonicalDate(evidence.expires_at);
  const now = validDate((options.now ?? (() => new Date()))());
  const { content_sha256: contentSha256, ...payload } = evidence;
  if (expiresAt.getTime() - collectedAt.getTime() < 1 ||
      expiresAt.getTime() - collectedAt.getTime() > eventLeaseReclaimEvidenceMaximumValidityMs ||
      now.getTime() < collectedAt.getTime() || now.getTime() >= expiresAt.getTime() ||
      digest(payload) !== contentSha256) {
    throw new Error("Event lease reclaim evidence is invalid");
  }
  return Object.freeze(evidence);
}

function digest(payload: z.infer<typeof payloadSchema>): string {
  return createHash("sha256")
    .update(`${eventLeaseReclaimEvidenceSchemaVersion}\n${canonicalMcpJSON(payload)}`, "utf8")
    .digest("hex");
}

function canonicalDate(value: string): Date {
  const parsed = validDate(new Date(value));
  if (parsed.toISOString() !== value) throw new Error("Event lease reclaim evidence time is invalid");
  return parsed;
}

function validDate(value: Date): Date {
  if (!Number.isFinite(value.getTime())) throw new Error("Event lease reclaim evidence time is invalid");
  return value;
}
