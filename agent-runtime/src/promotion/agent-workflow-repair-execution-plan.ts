import { createHash } from "node:crypto";

import { z } from "zod";

import { canonicalMcpJSON } from "../mcp/canonical-json.js";

const evidenceSchema = z.object({
  workflowId: z.string().trim().min(1), workflowRunId: z.string().trim().min(1),
  status: z.string().trim().min(1), revision: z.number().int().nonnegative()
}).strict();

const sha256Schema = z.string().regex(/^[a-f0-9]{64}$/u);

export interface AgentWorkflowRepairExecutionPlanInput {
  readonly proposalId: string;
  readonly proposalStatus: "approved";
  readonly proposalEvidenceSha256: string;
  readonly proposerId: string;
  readonly executorId: string;
  readonly executorGrantVersion: number;
  readonly changeTicketRef: string;
  readonly approverIds: readonly [string, string];
  readonly expectedCurrentProjection: AgentWorkflowRepairEvidence | null;
  readonly targetProjection: AgentWorkflowRepairEvidence;
  readonly rollbackProjection: AgentWorkflowRepairEvidence | null;
  readonly capturedAt: string;
  readonly expiresAt: string;
};

export interface AgentWorkflowRepairEvidence {
  readonly workflowId: string;
  readonly workflowRunId: string;
  readonly status: string;
  readonly revision: number;
}

export interface AgentWorkflowRepairExecutionPlan {
  readonly schemaVersion: "dipole.agent.workflow-repair-execution-plan.v1";
  readonly mode: "dry_run";
  readonly planId: string;
  readonly proposalId: string;
  readonly proposalStatus: "approved";
  readonly proposalEvidenceSha256: string;
  readonly proposerId: string;
  readonly executorId: string;
  readonly executorGrantVersion: number;
  readonly changeTicketRef: string;
  readonly approverIds: readonly [string, string];
  readonly expectedCurrentProjection: AgentWorkflowRepairEvidence | null;
  readonly targetProjection: AgentWorkflowRepairEvidence;
  readonly rollbackProjection: AgentWorkflowRepairEvidence | null;
  readonly expectedCurrentSha256: string | null;
  readonly targetSha256: string;
  readonly rollbackSha256: string | null;
  readonly capturedAt: string;
  readonly expiresAt: string;
}

const inputSchema = z.object({
  proposalId: z.string().regex(/^repair:[a-f0-9]{64}$/u), proposalStatus: z.literal("approved"),
  proposalEvidenceSha256: sha256Schema,
  proposerId: z.string().trim().min(1).max(24), executorId: z.string().trim().min(1).max(24),
  executorGrantVersion: z.number().int().positive(), changeTicketRef: z.string().trim().min(1).max(128),
  approverIds: z.tuple([z.string().trim().min(1).max(24), z.string().trim().min(1).max(24)]),
  expectedCurrentProjection: evidenceSchema.nullable(), targetProjection: evidenceSchema,
  rollbackProjection: evidenceSchema.nullable(), capturedAt: z.string().datetime(), expiresAt: z.string().datetime()
}).strict();

export function createAgentWorkflowRepairExecutionPlan(
  input: AgentWorkflowRepairExecutionPlanInput,
  now = new Date()
): AgentWorkflowRepairExecutionPlan {
  const parsed = inputSchema.parse(input);
  const identities = [parsed.proposerId, parsed.executorId, ...parsed.approverIds];
  if (new Set(identities).size !== identities.length) throw new Error("Repair execution identities must be distinct");
  if (parsed.rollbackProjection === null && parsed.expectedCurrentProjection !== null) {
    throw new Error("Repair rollback projection must preserve the expected current projection");
  }
  if (parsed.rollbackProjection !== null && !sameEvidence(parsed.rollbackProjection, parsed.expectedCurrentProjection)) {
    throw new Error("Repair rollback projection must equal the expected current projection");
  }
  if (parsed.expectedCurrentProjection !== null &&
      (parsed.expectedCurrentProjection.workflowId !== parsed.targetProjection.workflowId ||
       parsed.expectedCurrentProjection.workflowRunId !== parsed.targetProjection.workflowRunId)) {
    throw new Error("Repair projections must bind to the same Workflow Run");
  }
  const capturedAt = Date.parse(parsed.capturedAt);
  const expiresAt = Date.parse(parsed.expiresAt);
  if (!Number.isFinite(capturedAt) || !Number.isFinite(expiresAt) || expiresAt <= capturedAt || expiresAt - capturedAt > 15 * 60 * 1000) {
    throw new Error("Repair execution plan must expire within 15 minutes");
  }
  if (capturedAt > now.getTime() + 60_000 || expiresAt <= now.getTime()) {
    throw new Error("Repair execution plan is outside its active window");
  }
  const body = {
    schemaVersion: "dipole.agent.workflow-repair-execution-plan.v1" as const, mode: "dry_run" as const,
    proposalId: parsed.proposalId, proposalStatus: parsed.proposalStatus,
    proposalEvidenceSha256: parsed.proposalEvidenceSha256, proposerId: parsed.proposerId,
    executorId: parsed.executorId, executorGrantVersion: parsed.executorGrantVersion,
    changeTicketRef: parsed.changeTicketRef, approverIds: parsed.approverIds,
    expectedCurrentProjection: parsed.expectedCurrentProjection, targetProjection: parsed.targetProjection,
    rollbackProjection: parsed.rollbackProjection,
    expectedCurrentSha256: digestEvidence(parsed.expectedCurrentProjection),
    targetSha256: digestEvidence(parsed.targetProjection)!, rollbackSha256: digestEvidence(parsed.rollbackProjection),
    capturedAt: parsed.capturedAt, expiresAt: parsed.expiresAt
  };
  const planId = `repair-plan:${sha256(canonicalMcpJSON(body))}`;
  return { ...body, planId };
}

function sameEvidence(left: AgentWorkflowRepairEvidence | null, right: AgentWorkflowRepairEvidence | null): boolean {
  return canonicalMcpJSON(left) === canonicalMcpJSON(right);
}

function digestEvidence(value: AgentWorkflowRepairEvidence | null): string | null {
  return value === null ? null : sha256(canonicalMcpJSON(value));
}

function sha256(value: string): string {
  return createHash("sha256").update(value, "utf8").digest("hex");
}
