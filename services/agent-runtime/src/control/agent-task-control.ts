import type { AgentTaskControlAuthorization } from "../capabilities/agent-capability-rpc.js";
import type { AgentTaskState } from "../task/agent-task-state.js";
import { validateElicitationResponse, type AgentElicitationValue } from "../task/agent-elicitation.js";

export type AgentTaskControlErrorCode = "invalid_argument" | "not_found" | "conflict";

export class AgentTaskControlError extends Error {
  constructor(readonly code: AgentTaskControlErrorCode, message: string) {
    super(message);
    this.name = "AgentTaskControlError";
  }
}

export interface AgentTaskControlAuthorizationPort {
  authorizeTaskControl(taskId: string, principalUserId: string, context?: { requestId?: string; traceId?: string }): Promise<AgentTaskControlAuthorization>;
  listAgentTaskTimeline?(taskId: string, principalUserId: string, afterSeq: bigint, limit: number, context?: { requestId?: string; traceId?: string }): Promise<AgentTaskTimeline>;
}

export interface AgentTaskTimeline {
  readonly schemaVersion: string;
  readonly taskId: string;
  readonly revision: bigint;
  readonly events: readonly {
    readonly eventSeq: bigint;
    readonly eventId: string;
    readonly runId: string;
    readonly kind: string;
    readonly status: string;
    readonly capabilityId: string;
    readonly approvalId: string;
    readonly artifactId: string;
    readonly occurredAtUnixMs: bigint;
  }[];
  readonly nextCursor: string;
}

export interface AgentTaskWorkflowControlPort {
  query(taskId: string): Promise<AgentTaskState>;
  cancel(taskId: string, reason: string): Promise<void>;
  resolveApproval(taskId: string, signal: {
    requestId: string;
    approvalId: string;
    decision: "approved" | "denied";
    actorUserId: string;
  }): Promise<void>;
  provideInput(taskId: string, signal: { requestId: string; value: AgentElicitationValue }): Promise<void>;
}

export interface AgentTaskControlIdentity {
  taskId: string;
  principalUserId: string;
  requestId?: string;
  traceId?: string;
}

export class AgentTaskControlService {
  constructor(
    private readonly authorization: AgentTaskControlAuthorizationPort,
    private readonly workflows: AgentTaskWorkflowControlPort
  ) {}

  async getTask(input: AgentTaskControlIdentity): Promise<AgentTaskState & {
    persistentStatus: string;
    workflowProjection: { outcome: "match" | "missing" | "stale" | "ahead" | "conflict"; status?: string; revision?: number };
  }> {
    const authorization = await this.authorize(input);
    let state: AgentTaskState;
    try {
      state = await this.workflows.query(input.taskId);
    } catch (error) {
      const persisted = terminalWorkflowProjectionState(authorization, error);
      if (persisted === undefined) throw error;
      state = persisted;
    }
    if (state.taskId !== input.taskId) {
      throw new AgentTaskControlError("conflict", "Agent Task Workflow returned a conflicting binding");
    }
    return {
      ...state,
      persistentStatus: authorization.taskStatus,
      workflowProjection: reconcileWorkflowProjection(authorization, state)
    };
  }

  async getTimeline(input: AgentTaskControlIdentity & { afterSeq: bigint; limit: number }): Promise<AgentTaskTimeline> {
    if (!Number.isInteger(input.limit) || input.limit < 1 || input.limit > 100 || input.afterSeq < 0n) {
      throw new AgentTaskControlError("invalid_argument", "Agent Task Timeline pagination is invalid");
    }
    if (this.authorization.listAgentTaskTimeline === undefined) {
      throw new AgentTaskControlError("conflict", "Agent Task Timeline is unavailable");
    }
    return this.authorization.listAgentTaskTimeline(input.taskId, input.principalUserId, input.afterSeq, input.limit, {
      ...(input.requestId === undefined ? {} : { requestId: input.requestId }),
      ...(input.traceId === undefined ? {} : { traceId: input.traceId })
    });
  }

  async cancelTask(input: AgentTaskControlIdentity & { reason?: string }): Promise<void> {
    await this.authorize(input);
    const state = await this.workflows.query(input.taskId);
    if (state.taskId !== input.taskId) {
      throw new AgentTaskControlError("conflict", "Agent Task Workflow returned a conflicting binding");
    }
    if (state.status === "completed" || state.status === "failed" || state.status === "cancelled") {
      throw new AgentTaskControlError("conflict", `Agent Task is already ${state.status}`);
    }
    const reason = (input.reason?.trim() || "user_cancelled").slice(0, 256);
    await this.workflows.cancel(input.taskId, reason);
  }

  async resolveApproval(input: AgentTaskControlIdentity & { approvalId: string; decision: "approved" | "denied" }): Promise<void> {
    await this.authorize(input);
    const state = await this.workflows.query(input.taskId);
    if (state.status !== "waiting_approval" || state.pending?.kind !== "approval" || state.pending.approvalId !== input.approvalId) {
      throw new AgentTaskControlError("conflict", "Agent Task approval is no longer pending");
    }
    await this.workflows.resolveApproval(input.taskId, {
      requestId: state.pending.requestId,
      approvalId: state.pending.approvalId,
      decision: input.decision,
      actorUserId: input.principalUserId
    });
  }

  async provideInput(input: AgentTaskControlIdentity & { requestId: string; value: unknown }): Promise<void> {
    await this.authorize(input);
    const state = await this.workflows.query(input.taskId);
    if (state.status !== "waiting_input" || state.pending?.kind !== "input" || state.pending.requestId !== input.requestId) {
      throw new AgentTaskControlError("conflict", "Agent Task input is no longer pending");
    }
    let value: AgentElicitationValue;
    try { value = validateElicitationResponse(state.pending.form, input.value); }
    catch (error) { throw new AgentTaskControlError("invalid_argument", error instanceof Error ? error.message : "Agent Task input is invalid"); }
    await this.workflows.provideInput(input.taskId, { requestId: state.pending.requestId, value });
  }

  private async authorize(input: AgentTaskControlIdentity): Promise<AgentTaskControlAuthorization> {
    const taskId = input.taskId.trim();
    const principalUserId = input.principalUserId.trim();
    if (taskId.length === 0 || principalUserId.length === 0) {
      throw new AgentTaskControlError("invalid_argument", "Agent Task and principal are required");
    }
    const authorization = await this.authorization.authorizeTaskControl(taskId, principalUserId, {
      ...(input.requestId === undefined ? {} : { requestId: input.requestId }),
      ...(input.traceId === undefined ? {} : { traceId: input.traceId })
    });
    if (authorization.taskId !== taskId) {
      throw new AgentTaskControlError("conflict", "Core returned a conflicting Agent Task binding");
    }
    return authorization;
  }
}

// Temporal does not serve Workflow Queries after a terminal execution has
// closed. Core's owner-authorized projection remains the durable read model.
function terminalWorkflowProjectionState(
  authorization: AgentTaskControlAuthorization,
  error: unknown
): AgentTaskState | undefined {
  if (!workflowUnavailable(error)) return undefined;
  const projection = authorization.workflow;
  if (projection === undefined || !terminalTaskStatus(projection.workflowStatus)) return undefined;
  return {
    taskId: authorization.taskId,
    status: projection.workflowStatus,
    revision: projection.workflowRevision
  };
}

function workflowUnavailable(error: unknown): boolean {
  const code = typeof error === "object" && error !== null && "code" in error ? Number(error.code) : undefined;
  return code === 5 || error instanceof Error && error.name === "WorkflowNotFoundError";
}

function terminalTaskStatus(value: string): value is Extract<AgentTaskState["status"], "completed" | "failed" | "cancelled"> {
  return value === "completed" || value === "failed" || value === "cancelled";
}

function reconcileWorkflowProjection(authorization: AgentTaskControlAuthorization, state: AgentTaskState): {
  outcome: "match" | "missing" | "stale" | "ahead" | "conflict";
  status?: string;
  revision?: number;
} {
  const projection = authorization.workflow;
  if (projection === undefined) return { outcome: "missing" };
  const evidence = { status: projection.workflowStatus, revision: projection.workflowRevision };
  if (projection.workflowStatus === state.status && projection.workflowRevision === state.revision) return { outcome: "match", ...evidence };
  if (projection.workflowRevision < state.revision) return { outcome: "stale", ...evidence };
  if (projection.workflowRevision > state.revision) return { outcome: "ahead", ...evidence };
  return { outcome: "conflict", ...evidence };
}
