import type {
  AgentRunAdmissionRequest,
  AgentRunIdentity,
  AgentRunTerminalStatus
} from "../capabilities/agent-capability-rpc.js";
import type { AgentTaskLifecycleActivities } from "./agent-task-activities.js";
import { AgentTelemetry } from "../observability/agent-telemetry.js";

export interface PersistentAgentRunLifecyclePort {
  admitRun(input: AgentRunAdmissionRequest): Promise<AgentRunIdentity>;
  finish(
    taskId: string,
    runId: string,
    runStatus: AgentRunTerminalStatus,
    lastError: string,
    context?: { requestId?: string; traceId?: string }
  ): Promise<void>;
  requestApproval(taskId: string, runId: string, approval: Parameters<AgentTaskLifecycleActivities["requestAgentTaskApproval"]>[0]["approval"], context?: { requestId?: string; traceId?: string }): Promise<void>;
  resolveApproval(taskId: string, runId: string, approvalId: string, decision: "approved" | "denied", actorUserId: string, context?: { requestId?: string; traceId?: string }): Promise<void>;
  projectTaskWorkflowState(input: {
    taskId: string;
    runId: string;
    workflowId: string;
    workflowRunId: string;
    workflowStatus: string;
    workflowRevision: number;
  }, context?: { requestId?: string; traceId?: string }): Promise<unknown>;
}

export function createPersistentAgentTaskLifecycleActivities(
  lifecycle: PersistentAgentRunLifecyclePort,
  telemetry: Pick<AgentTelemetry, "withSpan"> = new AgentTelemetry()
): AgentTaskLifecycleActivities {
  return {
    async admitAgentTask(input) {
      if (input.admission === undefined) {
        throw new Error("Persistent Agent Task admission input is required");
      }
      const admitted = await telemetry.withSpan("agent.task", {
        taskId: input.taskId,
        attributes: { "dipole.agent.task.phase": "admit", "dipole.agent.trigger.type": input.admission.triggerType }
      }, async span => {
        const value = await lifecycle.admitRun(input.admission!);
        span.setAttribute("dipole.agent.run.id", value.runId);
        span.setAttribute("dipole.agent.run.status", value.runStatus);
        return value;
      });
      if (admitted.taskId !== input.taskId) {
        throw new Error("Persistent Agent Task admission returned a different Task ID");
      }
      return admitted;
    },
    async finishAgentTask(input) {
      const context = {
        ...(input.requestId === undefined ? {} : { requestId: input.requestId }),
        ...(input.traceId === undefined ? {} : { traceId: input.traceId })
      };
      await telemetry.withSpan("agent.task", {
        taskId: input.taskId, runId: input.runId,
        attributes: { "dipole.agent.task.phase": "finish", "dipole.agent.run.status": input.runStatus }
      }, async () => lifecycle.finish(input.taskId, input.runId, input.runStatus, input.lastError, context));
    },
    async projectAgentTaskState(input) {
      await lifecycle.projectTaskWorkflowState({
        taskId: input.taskId,
        runId: input.runId,
        workflowId: input.workflowId,
        workflowRunId: input.workflowRunId,
        workflowStatus: input.workflowStatus,
        workflowRevision: input.workflowRevision
      }, correlation(input));
    },
    async requestAgentTaskApproval(input) {
      await telemetry.withSpan("agent.approval", {
        taskId: input.taskId, runId: input.runId,
        attributes: { "dipole.agent.approval.phase": "request", "dipole.agent.capability.id": input.approval.capabilityId }
      }, async () => lifecycle.requestApproval(input.taskId, input.runId, input.approval, correlation(input)));
    },
    async resolveAgentTaskApproval(input) {
      await telemetry.withSpan("agent.approval", {
        taskId: input.taskId, runId: input.runId,
        attributes: { "dipole.agent.approval.phase": "resolve", "dipole.agent.approval.decision": input.decision }
      }, async () => lifecycle.resolveApproval(
        input.taskId, input.runId, input.approvalId, input.decision, input.actorUserId, correlation(input)
      ));
    }
  };
}

function correlation(input: { requestId?: string; traceId?: string }) {
  return {
    ...(input.requestId === undefined ? {} : { requestId: input.requestId }),
    ...(input.traceId === undefined ? {} : { traceId: input.traceId })
  };
}
