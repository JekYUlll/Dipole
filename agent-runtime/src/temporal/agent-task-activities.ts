import type { AgentTaskResume } from "../task/agent-task-state.js";
import type { AgentRunTerminalStatus } from "../capabilities/agent-capability-rpc.js";
import type { AgentApprovalBinding } from "../capabilities/agent-capability-rpc.js";
import type { AgentTaskWorkflowHistoryInput, AgentTaskWorkflowInput } from "./temporal-task-client.js";
import type { AgentElicitationForm, AgentElicitationSource } from "../task/agent-elicitation.js";

export interface AgentTaskActivityInput extends AgentTaskWorkflowInput {
  runId: string;
  step: number;
  checkpoint?: unknown;
  resume?: AgentTaskResume;
}

export type AgentTaskDirective =
  | { kind: "continue"; checkpoint?: unknown }
  | { kind: "wait_input"; requestId: string; prompt: string; form: AgentElicitationForm; source?: AgentElicitationSource; expiresAtUnixMs: number; checkpoint?: unknown }
  | { kind: "wait_approval"; requestId: string; summary: string; approval: AgentApprovalBinding; checkpoint?: unknown }
  | { kind: "complete"; output: unknown }
  | { kind: "failed"; message: string };

export interface AgentTaskActivities {
  executeAgentTaskStep(input: AgentTaskActivityInput): Promise<AgentTaskDirective>;
}

export interface AgentTaskRunBinding {
  taskId: string;
  runId: string;
  runStatus: "running" | "completed";
}

export interface AgentTaskFinishInput {
  taskId: string;
  runId: string;
  runStatus: AgentRunTerminalStatus;
  lastError: string;
  requestId?: string;
  traceId?: string;
}

export interface AgentTaskProjectionInput {
  taskId: string;
  runId: string;
  workflowId: string;
  workflowRunId: string;
  workflowStatus: string;
  workflowRevision: number;
  requestId?: string;
  traceId?: string;
}

export interface AgentTaskLifecycleActivities {
  admitAgentTask(input: AgentTaskWorkflowHistoryInput): Promise<AgentTaskRunBinding>;
  finishAgentTask(input: AgentTaskFinishInput): Promise<void>;
  projectAgentTaskState(input: AgentTaskProjectionInput): Promise<void>;
  requestAgentTaskApproval(input: { taskId: string; runId: string; approval: AgentApprovalBinding; requestId?: string; traceId?: string }): Promise<void>;
  resolveAgentTaskApproval(input: { taskId: string; runId: string; approvalId: string; decision: "approved" | "denied"; actorUserId: string; requestId?: string; traceId?: string }): Promise<void>;
}

export type AgentTaskWorkerActivities = AgentTaskActivities & AgentTaskLifecycleActivities;

export const foundationAgentTaskActivities: AgentTaskWorkerActivities = {
  async admitAgentTask(input): Promise<AgentTaskRunBinding> {
    return { taskId: input.taskId, runId: `foundation:${input.taskId}`, runStatus: "running" };
  },
  async finishAgentTask(): Promise<void> {},
  async projectAgentTaskState(): Promise<void> {},
  async requestAgentTaskApproval(): Promise<void> {},
  async resolveAgentTaskApproval(): Promise<void> {},
  async executeAgentTaskStep(): Promise<AgentTaskDirective> {
    return { kind: "failed", message: "Temporal Agent Task execution is not connected" };
  }
};
