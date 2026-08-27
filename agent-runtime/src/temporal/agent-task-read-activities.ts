import { executionContextSchema } from "../runtime/execution-context.js";
import {
  agentEventSchema,
  agentRunId,
  agentTaskId,
  executeShadowPlan,
  type ShadowPlanExecutionDependencies
} from "../events/shadow-processor.js";
import type { AgentTaskActivities } from "./agent-task-activities.js";

export function createTemporalReadStepActivities(
  dependencies: ShadowPlanExecutionDependencies
): AgentTaskActivities {
  return {
    async executeAgentTaskStep(input) {
      if (input.step !== 0 || input.admission === undefined || input.shadowEvent === undefined) {
        throw new Error("Temporal read Step requires step 0 with trusted admission and shadow event");
      }
      const event = agentEventSchema.parse(input.shadowEvent);
      const admission = input.admission;
      const expectedTaskId = agentTaskId({
        tenantId: admission.tenantId,
        agentUuid: admission.agentId,
        triggerType: event.eventType,
        triggerRef: event.aggregateId
      });
      if (input.taskId !== expectedTaskId || input.runId !== agentRunId(expectedTaskId) ||
          admission.eventId !== event.eventId || admission.triggerType !== event.eventType ||
          admission.triggerRef !== event.aggregateId) {
        throw new Error("Temporal read Step Task, Run, admission, and event binding mismatch");
      }
      const context = executionContextSchema.parse({
        tenantId: admission.tenantId,
        principalUuid: admission.principalUserId,
        agentUuid: admission.agentId,
        taskId: input.taskId,
        runId: input.runId,
        mode: "shadow",
        permissions: ["conversation.list", "conversation.read"],
        resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["read", "list"] }],
        approvedCapabilities: [],
        eventId: event.eventId,
        ...(admission.requestId === undefined ? {} : { requestId: admission.requestId }),
        ...(admission.traceId === undefined ? {} : { traceId: admission.traceId })
      });
      const plan = await executeShadowPlan(event, context, dependencies);
      return {
        kind: "complete",
        output: { summary: plan.summary, stepCount: plan.steps.length }
      };
    }
  };
}
