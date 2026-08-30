import { z } from "zod";

export const agentMemoryTypes = [
  "working",
  "episodic",
  "semantic",
  "procedural",
  "observational",
] as const;

export const agentMemoryTypeSchema = z.enum(agentMemoryTypes);
export const observationalCandidateTypeSchema = z.literal("observational");

export type AgentMemoryType = z.infer<typeof agentMemoryTypeSchema>;
export type ObservationalCandidateType = z.infer<typeof observationalCandidateTypeSchema>;

export interface AgentMemoryTypePolicy {
  readonly type: AgentMemoryType;
  readonly durable: boolean;
  readonly taskScoped: boolean;
  readonly requiresReview: boolean;
}

const policies: Readonly<Record<AgentMemoryType, AgentMemoryTypePolicy>> = {
  working: { type: "working", durable: false, taskScoped: true, requiresReview: false },
  episodic: { type: "episodic", durable: true, taskScoped: false, requiresReview: true },
  semantic: { type: "semantic", durable: true, taskScoped: false, requiresReview: true },
  procedural: { type: "procedural", durable: true, taskScoped: false, requiresReview: true },
  observational: { type: "observational", durable: true, taskScoped: false, requiresReview: true },
};

export function parseAgentMemoryType(value: unknown): AgentMemoryType {
  return agentMemoryTypeSchema.parse(value);
}

export function getAgentMemoryTypePolicy(value: unknown): AgentMemoryTypePolicy {
  return policies[parseAgentMemoryType(value)];
}

/**
 * Resolves a reviewed candidate's storage type without granting write authority.
 * The caller must separately verify review, ownership, and promotion permissions.
 */
export function validateMemoryTypeTransition(
  candidateType: unknown,
  requestedType: unknown,
): AgentMemoryType {
  observationalCandidateTypeSchema.parse(candidateType);
  return parseAgentMemoryType(requestedType);
}
