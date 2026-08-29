import { createHash } from "node:crypto";

import { z } from "zod";

import { canonicalMcpJSON } from "../mcp/canonical-json.js";

export const externalMcpCredentialLifecycleEvidenceSchemaVersion =
  "dipole.agent.external-mcp-credential-lifecycle-drill.v1" as const;
export const externalMcpCredentialLifecycleEvidenceMaximumValidityMs = 24 * 60 * 60 * 1_000;

const outcomeSchema = z.object({
  initial_credential_verified: z.literal(true),
  rotated_credential_verified: z.literal(true),
  old_version_revoked_before_transport: z.literal(true),
  restart_recovered: z.literal(true),
  active_version_revoked_before_transport: z.literal(true),
  transport_open_count: z.literal(3),
  transport_close_count: z.literal(3),
  inflight_revocation_authority: z.literal(false)
}).strict();

const payloadSchema = outcomeSchema.extend({
  schema_version: z.literal(externalMcpCredentialLifecycleEvidenceSchemaVersion),
  outcome: z.literal("passed"),
  isolation: z.literal("disposable_encrypted_files_and_injected_transport"),
  collected_at: z.string(),
  expires_at: z.string(),
  production_authority: z.literal(false)
}).strict();

const evidenceSchema = payloadSchema.extend({
  content_sha256: z.string().regex(/^[a-f0-9]{64}$/)
}).strict();

export type ExternalMcpCredentialLifecycleEvidence = z.infer<typeof evidenceSchema>;
export type ExternalMcpCredentialLifecycleEvidenceOutcome = z.infer<typeof outcomeSchema>;

interface EvidenceClockOptions {
  readonly now?: () => Date;
}

interface EvidenceCreationOptions extends EvidenceClockOptions {
  readonly validityMs?: number;
}

export function createExternalMcpCredentialLifecycleEvidence(
  outcome: ExternalMcpCredentialLifecycleEvidenceOutcome,
  options: EvidenceCreationOptions = {}
): ExternalMcpCredentialLifecycleEvidence {
  const validityMs = options.validityMs ?? externalMcpCredentialLifecycleEvidenceMaximumValidityMs;
  if (!Number.isSafeInteger(validityMs) || validityMs < 1 ||
      validityMs > externalMcpCredentialLifecycleEvidenceMaximumValidityMs) {
    throw new Error("External MCP credential lifecycle evidence validity is invalid");
  }
  const collectedAt = validDate((options.now ?? (() => new Date()))());
  const payload = payloadSchema.parse({
    schema_version: externalMcpCredentialLifecycleEvidenceSchemaVersion,
    outcome: "passed",
    isolation: "disposable_encrypted_files_and_injected_transport",
    collected_at: collectedAt.toISOString(),
    expires_at: new Date(collectedAt.getTime() + validityMs).toISOString(),
    ...outcomeSchema.parse(outcome),
    production_authority: false
  });
  return Object.freeze({ ...payload, content_sha256: digest(payload) });
}

export function parseExternalMcpCredentialLifecycleEvidence(
  value: unknown,
  options: EvidenceClockOptions = {}
): ExternalMcpCredentialLifecycleEvidence {
  const evidence = evidenceSchema.parse(value);
  const collectedAt = canonicalDate(evidence.collected_at);
  const expiresAt = canonicalDate(evidence.expires_at);
  const now = validDate((options.now ?? (() => new Date()))());
  const { content_sha256: contentSha256, ...payload } = evidence;
  const validityMs = expiresAt.getTime() - collectedAt.getTime();
  if (validityMs < 1 || validityMs > externalMcpCredentialLifecycleEvidenceMaximumValidityMs ||
      now.getTime() < collectedAt.getTime() || now.getTime() >= expiresAt.getTime() ||
      digest(payload) !== contentSha256) {
    throw new Error("External MCP credential lifecycle evidence is invalid");
  }
  return Object.freeze(evidence);
}

function digest(payload: z.infer<typeof payloadSchema>): string {
  return createHash("sha256")
    .update(`${externalMcpCredentialLifecycleEvidenceSchemaVersion}\n${canonicalMcpJSON(payload)}`, "utf8")
    .digest("hex");
}

function canonicalDate(value: string): Date {
  const parsed = validDate(new Date(value));
  if (parsed.toISOString() !== value) throw new Error("External MCP credential lifecycle evidence time is invalid");
  return parsed;
}

function validDate(value: Date): Date {
  if (!Number.isFinite(value.getTime())) throw new Error("External MCP credential lifecycle evidence time is invalid");
  return value;
}
