import { validateElicitationForm, validateElicitationResponse, type AgentElicitationForm, type AgentElicitationValue } from "./agent-elicitation.js";

export type AgentTaskStatus =
  | "created"
  | "running"
  | "waiting_input"
  | "waiting_approval"
  | "completed"
  | "failed"
  | "cancelled";

export type AgentTaskPending =
  | { kind: "input"; requestId: string; prompt: string; form: AgentElicitationForm; expiresAtUnixMs: number }
  | { kind: "approval"; requestId: string; approvalId: string; summary: string; expiresAtUnixMs: number };

export type AgentTaskResume =
  | { kind: "input"; requestId: string; value: AgentElicitationValue }
  | { kind: "approval"; requestId: string; approvalId: string; decision: "approved" };

export interface AgentTaskState {
  taskId: string;
  status: AgentTaskStatus;
  revision: number;
  pending?: AgentTaskPending;
  resume?: AgentTaskResume;
  output?: unknown;
  failure?: { message: string };
  cancellation?: { reason: string; requestId?: string };
}

export type AgentTaskTransition =
  | { type: "start" }
  | { type: "request_input"; requestId: string; prompt: string; form: AgentElicitationForm; expiresAtUnixMs: number }
  | { type: "provide_input"; requestId: string; value: unknown }
  | { type: "request_approval"; requestId: string; approvalId: string; summary: string; expiresAtUnixMs: number }
  | { type: "resolve_approval"; requestId: string; decision: "approved" | "denied" }
  | { type: "expire_wait"; requestId: string }
  | { type: "complete"; output: unknown }
  | { type: "fail"; message: string }
  | { type: "cancel"; reason: string };

const terminalStatuses = new Set<AgentTaskStatus>(["completed", "failed", "cancelled"]);

export function createAgentTaskState(taskId: string): AgentTaskState {
  if (taskId.trim().length === 0) {
    throw new Error("Agent Task ID is required");
  }
  return { taskId, status: "created", revision: 0 };
}

export function transitionAgentTask(state: AgentTaskState, event: AgentTaskTransition): AgentTaskState {
  if (terminalStatuses.has(state.status)) {
    throw new Error(`Agent Task ${state.taskId} is terminal in ${state.status}`);
  }

  switch (event.type) {
    case "start":
      requireStatus(state, event.type, "created");
      return next(state, "running");
    case "request_input":
      requireStatus(state, event.type, "running");
      if (event.requestId.trim().length === 0 || event.requestId.length > 128 || event.prompt.trim().length === 0 || event.prompt.length > 2000) throw new Error("Agent Elicitation request identity or prompt is invalid");
      return { ...next(state, "waiting_input"), pending: { kind: "input", requestId: event.requestId, prompt: event.prompt, form: validateElicitationForm(event.form), expiresAtUnixMs: validDeadline(event.expiresAtUnixMs) } };
    case "provide_input": {
      requireStatus(state, event.type, "waiting_input");
      const pending = requirePending(state, "input", event.requestId);
      return {
        ...next(state, "running"),
        resume: { kind: "input", requestId: pending.requestId, value: validateElicitationResponse(pending.form, event.value) }
      };
    }
    case "request_approval":
      requireStatus(state, event.type, "running");
      return {
        ...next(state, "waiting_approval"),
        pending: { kind: "approval", requestId: event.requestId, approvalId: event.approvalId, summary: event.summary, expiresAtUnixMs: validDeadline(event.expiresAtUnixMs) }
      };
    case "resolve_approval": {
      requireStatus(state, event.type, "waiting_approval");
      const pending = requirePending(state, "approval", event.requestId);
      if (event.decision === "denied") {
        return {
          ...next(state, "cancelled"),
          cancellation: { reason: "approval_denied", requestId: pending.requestId }
        };
      }
      return {
        ...next(state, "running"),
        resume: { kind: "approval", requestId: pending.requestId, approvalId: pending.approvalId, decision: "approved" }
      };
    }
    case "expire_wait": {
      if (state.status !== "waiting_input" && state.status !== "waiting_approval") throw new Error(`Cannot expire wait while Agent Task is ${state.status}`);
      const pending = requirePending(state, state.status === "waiting_input" ? "input" : "approval", event.requestId);
      return {
        ...next(state, "cancelled"),
        cancellation: { reason: pending.kind === "input" ? "input_expired" : "approval_expired", requestId: pending.requestId }
      };
    }
    case "complete":
      requireStatus(state, event.type, "running");
      return { ...next(state, "completed"), output: event.output };
    case "fail":
      requireStatus(state, event.type, "running");
      return { ...next(state, "failed"), failure: { message: event.message } };
    case "cancel":
      return { ...next(state, "cancelled"), cancellation: { reason: event.reason } };
  }
}

function validDeadline(value: number): number {
  if (!Number.isSafeInteger(value) || value <= 0) throw new Error("Agent Task wait deadline is invalid");
  return value;
}

function next(state: AgentTaskState, status: AgentTaskStatus): AgentTaskState {
  return { taskId: state.taskId, status, revision: state.revision + 1 };
}

function requireStatus(state: AgentTaskState, transition: AgentTaskTransition["type"], expected: AgentTaskStatus): void {
  if (state.status !== expected) {
    throw new Error(`Cannot apply ${transition} while Agent Task is ${state.status}; expected ${expected}`);
  }
}

function requirePending<K extends AgentTaskPending["kind"]>(
  state: AgentTaskState,
  kind: K,
  requestId: string
): Extract<AgentTaskPending, { kind: K }> {
  const pending = state.pending;
  if (pending?.kind !== kind || pending.requestId !== requestId) {
    const expected = pending?.requestId ?? "no pending request";
    throw new Error(`Agent Task expected ${kind} request ${expected}, received ${requestId}`);
  }
  return pending as Extract<AgentTaskPending, { kind: K }>;
}
