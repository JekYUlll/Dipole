import { createHash } from "node:crypto";

import { z } from "zod";

const sha256Schema = z.string().regex(/^[a-f0-9]{64}$/);
const versionSchema = z.string().trim().min(2).max(128).regex(/^[A-Za-z0-9][A-Za-z0-9._:@/-]*$/);
const componentSchema = z.object({ version: versionSchema, sha256: sha256Schema }).strict();

export const agentReleaseManifestSchema = z.object({
  schemaVersion: z.literal("dipole.agent.release-manifest.v1"),
  candidateVersion: versionSchema,
  runtimeId: z.literal("dipole-agent"),
  stage: z.enum(["offline", "shadow", "user_gray"]),
  components: z.object({
    model: componentSchema,
    prompt: componentSchema,
    capabilitySchema: componentSchema,
    memoryPolicy: componentSchema
  }).strict(),
  offlineEvalSuiteSha256: sha256Schema,
  createdAt: z.iso.datetime().optional()
}).strict();

export type AgentReleaseManifest = z.infer<typeof agentReleaseManifestSchema>;
export type AgentReleaseStage = AgentReleaseManifest["stage"];

export function parseAgentReleaseManifest(value: unknown): AgentReleaseManifest {
  return agentReleaseManifestSchema.parse(value);
}

export function agentReleaseManifestSha256(value: unknown): string {
  return createHash("sha256").update(canonicalJSON(parseAgentReleaseManifest(value)), "utf8").digest("hex");
}

export function assertShadowPromotionBinding(
  value: unknown,
  candidateVersion: string,
  offlineEvalSuiteSha256: string
): AgentReleaseManifest {
  const manifest = parseAgentReleaseManifest(value);
  if (manifest.stage !== "shadow") throw new Error("Agent release manifest is not in shadow stage");
  if (manifest.candidateVersion !== candidateVersion) throw new Error("Agent release candidate version does not match promotion evidence");
  if (manifest.offlineEvalSuiteSha256 !== offlineEvalSuiteSha256) throw new Error("Agent release Eval Suite hash does not match promotion evidence");
  return manifest;
}

export function transitionAgentReleaseStage(value: unknown, nextStage: AgentReleaseStage): AgentReleaseManifest {
  const manifest = parseAgentReleaseManifest(value);
  const stages: readonly AgentReleaseStage[] = ["offline", "shadow", "user_gray"];
  const currentIndex = stages.indexOf(manifest.stage);
  const nextIndex = stages.indexOf(nextStage);
  if (Math.abs(currentIndex - nextIndex) !== 1) {
    throw new Error(`Agent release stage transition ${manifest.stage} -> ${nextStage} must be one step`);
  }
  return { ...manifest, stage: nextStage };
}

function canonicalJSON(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(canonicalJSON).join(",")}]`;
  if (value !== null && typeof value === "object") {
    return `{${Object.entries(value).sort(([left], [right]) => left.localeCompare(right)).map(([key, item]) => `${JSON.stringify(key)}:${canonicalJSON(item)}`).join(",")}}`;
  }
  return JSON.stringify(value);
}
