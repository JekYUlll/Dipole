import { createHash } from "node:crypto";

import { z } from "zod";

import type { AgentArtifactCreateInput, AgentArtifactRecord } from "../capabilities/agent-capability-rpc.js";
import {
  evaluateAgentShadowPromotionV2, parseAgentShadowPromotionEvidenceV2,
} from "./agent-shadow-promotion-policy.js";

const bindingSchema = z.object({
  schemaVersion: z.literal("dipole.agent.promotion-evidence-publication.v1"),
  tenantId: z.string().trim().min(1).max(64),
  taskId: z.string().trim().min(1).max(64),
  runId: z.string().trim().min(1).max(64),
  runtimeId: z.literal("dipole-agent"),
  definitionId: z.string().trim().min(1).max(64),
  definitionVersion: z.number().int().positive().safe(),
  requestId: z.string().trim().min(1).max(128).optional(),
  traceId: z.string().trim().min(1).max(128).optional()
}).strict();

export interface PromotionEvidencePublicationInput {
  readonly schemaVersion: "dipole.agent.promotion-evidence-publication.v1";
  readonly tenantId: string;
  readonly taskId: string;
  readonly runId: string;
  readonly runtimeId: "dipole-agent";
  readonly definitionId: string;
  readonly definitionVersion: number;
  readonly evidence: unknown;
  readonly requestId?: string;
  readonly traceId?: string;
}

export interface PromotionEvidenceReceipt {
  readonly schemaVersion: "dipole.agent.promotion-evidence-receipt.v1";
  readonly artifactId: string;
  readonly evidenceSHA256: string;
  readonly evalSuiteSHA256: string;
  readonly tenantId: string;
  readonly taskId: string;
  readonly runId: string;
  readonly runtimeId: "dipole-agent";
  readonly candidateVersion: string;
  readonly definitionId: string;
  readonly definitionVersion: number;
}

interface ArtifactPublisher {
  createArtifact(input: AgentArtifactCreateInput): Promise<AgentArtifactRecord>;
}

export class PromotionEvidencePublisher {
  constructor(private readonly artifacts: ArtifactPublisher) {}

  async publish(input: PromotionEvidencePublicationInput): Promise<PromotionEvidenceReceipt> {
    const binding = bindingSchema.parse({
      schemaVersion: input.schemaVersion,
      tenantId: input.tenantId, taskId: input.taskId, runId: input.runId, runtimeId: input.runtimeId,
      definitionId: input.definitionId, definitionVersion: input.definitionVersion,
      ...(input.requestId === undefined ? {} : { requestId: input.requestId }),
      ...(input.traceId === undefined ? {} : { traceId: input.traceId })
    });
    const evidence = parseAgentShadowPromotionEvidenceV2(input.evidence);
    const decision = evaluateAgentShadowPromotionV2(evidence);
    if (decision.decision !== "eligible") {
      throw new Error(`Agent promotion evidence is not eligible: ${decision.reasons.join(",")}`);
    }
    const metadata = {
      runtimeId: binding.runtimeId, candidateVersion: evidence.candidateVersion,
      definitionId: binding.definitionId, definitionVersion: binding.definitionVersion,
      evalSuiteSHA256: decision.offlineEvalSuiteSha256
    } as const;
    const content = Buffer.from(canonicalJSON({
      schemaVersion: "dipole.agent.promotion-evaluation.v1", runtimeId: binding.runtimeId,
      candidateVersion: evidence.candidateVersion,
      definition: { id: binding.definitionId, version: binding.definitionVersion },
      evidence, decision
    }), "utf8");
    const artifact = await this.artifacts.createArtifact({
      tenantId: binding.tenantId, taskId: binding.taskId, runId: binding.runId,
      artifactType: "promotion_evaluation", version: 1, title: "Agent Runtime promotion evaluation",
      mediaType: "application/json", content, metadata,
      ...(binding.requestId === undefined ? {} : { requestId: binding.requestId }),
      ...(binding.traceId === undefined ? {} : { traceId: binding.traceId })
    });
    const evidenceSHA256 = createHash("sha256").update(content).digest("hex");
    if (artifact.contentSha256 !== evidenceSHA256 || artifact.taskId !== binding.taskId || artifact.runId !== binding.runId ||
        artifact.artifactType !== "promotion_evaluation" || artifact.version !== 1 || canonicalJSON(artifact.metadata) !== canonicalJSON(metadata)) {
      throw new Error("Agent promotion evidence Artifact receipt conflicts with the publication binding");
    }
    return {
      schemaVersion: "dipole.agent.promotion-evidence-receipt.v1", artifactId: artifact.artifactId,
      evidenceSHA256, evalSuiteSHA256: decision.offlineEvalSuiteSha256,
      tenantId: binding.tenantId, taskId: binding.taskId, runId: binding.runId, runtimeId: binding.runtimeId,
      candidateVersion: evidence.candidateVersion, definitionId: binding.definitionId, definitionVersion: binding.definitionVersion
    };
  }
}

function canonicalJSON(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(canonicalJSON).join(",")}]`;
  if (value !== null && typeof value === "object") {
    return `{${Object.entries(value).sort(([left], [right]) => left < right ? -1 : left > right ? 1 : 0).map(([key, item]) => `${JSON.stringify(key)}:${canonicalJSON(item)}`).join(",")}}`;
  }
  return JSON.stringify(value);
}
