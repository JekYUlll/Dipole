import type {
  AgentRunAdmissionRequest,
  AgentRunIdentity,
  AgentRunTerminalStatus
} from "../capabilities/agent-capability-rpc.js";
import type { AgentTaskLifecycleActivities } from "./agent-task-activities.js";

export interface PersistentAgentRunLifecyclePort {
  admitRun(input: AgentRunAdmissionRequest): Promise<AgentRunIdentity>;
  finish(
    taskId: string,
    runId: string,
    runStatus: AgentRunTerminalStatus,
    lastError: string,
    context?: { requestId?: string; traceId?: string }
  ): Promise<void>;
}

export function createPersistentAgentTaskLifecycleActivities(
  lifecycle: PersistentAgentRunLifecyclePort
): AgentTaskLifecycleActivities {
  return {
    async admitAgentTask(input) {
      if (input.admission === undefined) {
        throw new Error("Persistent Agent Task admission input is required");
      }
      const admitted = await lifecycle.admitRun(input.admission);
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
      await lifecycle.finish(
        input.taskId,
        input.runId,
        input.runStatus,
        input.lastError,
        context
      );
    }
  };
}
