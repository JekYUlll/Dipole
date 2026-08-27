import type { AgentTaskResume } from "../task/agent-task-state.js";
import type { AgentRunTerminalStatus } from "../capabilities/agent-capability-rpc.js";
import type { AgentTaskWorkflowInput } from "./temporal-task-client.js";

export interface AgentTaskActivityInput extends AgentTaskWorkflowInput {
  step: number;
  checkpoint?: unknown;
  resume?: AgentTaskResume;
}

export type AgentTaskDirective =
  | { kind: "continue"; checkpoint?: unknown }
  | { kind: "wait_input"; requestId: string; prompt: string; checkpoint?: unknown }
  | { kind: "wait_approval"; requestId: string; summary: string; checkpoint?: unknown }
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

export interface AgentTaskLifecycleActivities {
  admitAgentTask(input: AgentTaskWorkflowInput): Promise<AgentTaskRunBinding>;
  finishAgentTask(input: AgentTaskFinishInput): Promise<void>;
}

export type AgentTaskWorkerActivities = AgentTaskActivities & AgentTaskLifecycleActivities;

export const foundationAgentTaskActivities: AgentTaskWorkerActivities = {
  async admitAgentTask(input): Promise<AgentTaskRunBinding> {
    return { taskId: input.taskId, runId: `foundation:${input.taskId}`, runStatus: "running" };
  },
  async finishAgentTask(): Promise<void> {},
  async executeAgentTaskStep(): Promise<AgentTaskDirective> {
    return { kind: "failed", message: "Temporal Agent Task execution is not connected" };
  }
};
