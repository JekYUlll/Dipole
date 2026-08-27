import type { AgentTaskResume } from "../task/agent-task-state.js";
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

export const foundationAgentTaskActivities: AgentTaskActivities = {
  async executeAgentTaskStep(): Promise<AgentTaskDirective> {
    return { kind: "failed", message: "Temporal Agent Task execution is not connected" };
  }
};
