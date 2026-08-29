import { createHash } from "node:crypto";
import { z } from "zod";

type RepairableOutcome = "missing" | "stale" | "ahead" | "conflict";

interface WorkflowEvidence {
  workflowId: string;
  workflowRunId: string;
  status: string;
  revision: number;
}

export interface AgentWorkflowRepairProposalInput {
  taskId: string;
  outcome: RepairableOutcome | "unavailable";
  operatorId: string;
  ticketRef: string;
  reason: string;
  projected?: WorkflowEvidence;
  temporal?: WorkflowEvidence;
  proposedAt: string;
  expiresAt: string;
}

export interface AgentWorkflowRepairProposal {
  schemaVersion: "dipole.agent.workflow-repair-proposal.v1";
  proposalId: string;
  status: "proposed";
  action: "reproject_from_temporal";
  taskId: string;
  outcome: RepairableOutcome;
  operatorId: string;
  ticketRef: string;
  reason: string;
  projected?: WorkflowEvidence;
  temporal: WorkflowEvidence;
  proposedAt: string;
  expiresAt: string;
  evidenceSha256: string;
}

const workflowEvidenceSchema = z.object({
  workflowId: z.string().trim().min(1), workflowRunId: z.string().trim().min(1),
  status: z.string().trim().min(1), revision: z.number().int().nonnegative()
}).strict();

const repairProposalInputSchema = z.object({
  taskId: z.string().trim().min(1),
  outcome: z.enum(["missing", "stale", "ahead", "conflict", "unavailable"]),
  operatorId: z.string().trim().min(1), ticketRef: z.string().trim().min(1), reason: z.string().trim().min(1),
  projected: workflowEvidenceSchema.optional(), temporal: workflowEvidenceSchema.optional(),
  proposedAt: z.string().datetime(), expiresAt: z.string().datetime()
}).strict();

export function parseAgentWorkflowRepairProposalInput(value: unknown): AgentWorkflowRepairProposalInput {
  const parsed = repairProposalInputSchema.parse(value);
  return {
    taskId: parsed.taskId, outcome: parsed.outcome, operatorId: parsed.operatorId,
    ticketRef: parsed.ticketRef, reason: parsed.reason,
    ...(parsed.projected === undefined ? {} : { projected: parsed.projected }),
    ...(parsed.temporal === undefined ? {} : { temporal: parsed.temporal }),
    proposedAt: parsed.proposedAt, expiresAt: parsed.expiresAt
  };
}

export function createAgentWorkflowRepairProposal(input: AgentWorkflowRepairProposalInput): AgentWorkflowRepairProposal {
  if (input.outcome === "unavailable") throw new Error("Agent Workflow repair requires a repairable discrepancy");
  const taskId = required(input.taskId, "Task ID");
  const operatorId = required(input.operatorId, "operator ID");
  const ticketRef = required(input.ticketRef, "ticket reference");
  const reason = required(input.reason, "reason");
  if (input.temporal === undefined) throw new Error("Agent Workflow repair requires Temporal evidence");
  const proposedAt = timestamp(input.proposedAt);
  const expiresAt = timestamp(input.expiresAt);
  if (expiresAt <= proposedAt || expiresAt-proposedAt > 60 * 60 * 1000) {
    throw new Error("Agent Workflow repair proposal must expire within one hour");
  }
  const evidence = {
    schemaVersion: "dipole.agent.workflow-repair-evidence.v1",
    taskId, outcome: input.outcome, operatorId, ticketRef, reason,
    projected: input.projected ?? null,
    temporal: input.temporal,
    proposedAt: new Date(proposedAt).toISOString(),
    expiresAt: new Date(expiresAt).toISOString()
  };
  const evidenceSha256 = sha256(JSON.stringify(evidence));
  return {
    schemaVersion: "dipole.agent.workflow-repair-proposal.v1",
    proposalId: `repair:${evidenceSha256}`,
    status: "proposed",
    action: "reproject_from_temporal",
    taskId, outcome: input.outcome, operatorId, ticketRef, reason,
    ...(input.projected === undefined ? {} : { projected: input.projected }),
    temporal: input.temporal,
    proposedAt: evidence.proposedAt,
    expiresAt: evidence.expiresAt,
    evidenceSha256
  };
}

function required(value: string, name: string): string {
  const normalized = value.trim();
  if (normalized === "") throw new Error(`Agent Workflow repair ${name} is required`);
  return normalized;
}

function timestamp(value: string): number {
  const parsed = Date.parse(value);
  if (!Number.isFinite(parsed)) throw new Error("Agent Workflow repair timestamp is invalid");
  return parsed;
}

function sha256(value: string): string {
  return createHash("sha256").update(value).digest("hex");
}
