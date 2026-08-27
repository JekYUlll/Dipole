import { executionContextSchema } from "../runtime/execution-context.js";
import {
  agentEventSchema,
  agentRunId,
  agentTaskId,
  executeShadowPlan,
  type ShadowPlanExecutionDependencies
} from "../events/shadow-processor.js";
import type { AgentTaskActivities } from "./agent-task-activities.js";
import type { AgentArtifactCreateInput, AgentArtifactRecord } from "../capabilities/agent-capability-rpc.js";

interface AgentArtifactWriter {
  createArtifact(input: AgentArtifactCreateInput): Promise<AgentArtifactRecord>;
}

export function createTemporalReadStepActivities(
  dependencies: ShadowPlanExecutionDependencies & { readonly artifacts?: AgentArtifactWriter }
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
	  const artifact = dependencies.artifacts === undefined ? undefined : await dependencies.artifacts.createArtifact({
	    tenantId: context.tenantId,
	    taskId: context.taskId,
	    runId: context.runId,
	    artifactType: "conversation_digest",
	    version: 1,
	    title: "Conversation digest",
	    mediaType: "text/markdown",
	    content: Buffer.from(`# Conversation digest\n\n${plan.summary.trim()}\n`, "utf8"),
	    metadata: { event_id: event.eventId, event_type: event.eventType, step_count: plan.steps.length },
	    ...(context.requestId === undefined ? {} : { requestId: context.requestId }),
	    ...(context.traceId === undefined ? {} : { traceId: context.traceId })
	  });
      return {
        kind: "complete",
		output: {
		  summary: plan.summary,
		  stepCount: plan.steps.length,
		  ...(artifact === undefined ? {} : { artifactId: artifact.artifactId, artifactVersion: artifact.version })
		}
      };
    }
  };
}
