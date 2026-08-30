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
import { AgentTelemetry } from "../observability/agent-telemetry.js";
import type { AgentRuntimeMode } from "../capabilities/agent-capability-rpc.js";
import type { ExecutionContext } from "../runtime/execution-context.js";

interface AgentArtifactWriter {
  createArtifact(input: AgentArtifactCreateInput): Promise<AgentArtifactRecord>;
}

interface AgentContextResolver {
  resolveMcpContext(taskId: string, runId: string, principalUserId: string, context?: { requestId?: string; traceId?: string }): Promise<ExecutionContext>;
}

export function createTemporalReadStepActivities(
  dependencies: ShadowPlanExecutionDependencies & {
    readonly artifacts?: AgentArtifactWriter;
    readonly runtimeMode?: AgentRuntimeMode;
    readonly contextResolver?: AgentContextResolver;
    readonly readPermissions?: readonly string[];
  }
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
      const runtimeMode = dependencies.runtimeMode ?? "shadow";
      if (input.taskId !== expectedTaskId || input.runId !== agentRunId(expectedTaskId, "dipole-agent", runtimeMode) ||
          admission.eventId !== event.eventId || admission.triggerType !== event.eventType ||
          admission.triggerRef !== event.aggregateId) {
        throw new Error("Temporal read Step Task, Run, admission, and event binding mismatch");
      }
      if (runtimeMode === "active" && dependencies.contextResolver === undefined) {
        throw new Error("Active Temporal read Step requires the Core Context resolver");
      }
      const context = dependencies.contextResolver === undefined
        ? executionContextSchema.parse({
          tenantId: admission.tenantId,
          principalUuid: admission.principalUserId,
          agentUuid: admission.agentId,
          taskId: input.taskId,
          runId: input.runId,
          mode: runtimeMode,
          permissions: dependencies.readPermissions ?? ["conversation.list", "conversation.read"],
          resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["read", "list"] }],
          approvedCapabilities: [],
          eventId: event.eventId,
          ...(admission.requestId === undefined ? {} : { requestId: admission.requestId }),
          ...(admission.traceId === undefined ? {} : { traceId: admission.traceId })
        })
        : await dependencies.contextResolver.resolveMcpContext(input.taskId, input.runId, admission.principalUserId, {
          ...(admission.requestId === undefined ? {} : { requestId: admission.requestId }),
          ...(admission.traceId === undefined ? {} : { traceId: admission.traceId })
        });
      if (context.mode !== runtimeMode || context.taskId !== input.taskId || context.runId !== input.runId || context.eventId !== event.eventId) {
        throw new Error("Core Context binding mismatch for Temporal read Step");
      }
      const telemetry = dependencies.telemetry ?? new AgentTelemetry();
      return telemetry.withSpan("agent.run", {
        taskId: context.taskId, runId: context.runId,
        attributes: { "dipole.agent.mode": context.mode, "dipole.agent.event.type": event.eventType }
      }, async span => {
        const plan = await executeShadowPlan(event, context, { ...dependencies, telemetry });
        const artifact = dependencies.artifacts === undefined ? undefined : await telemetry.withSpan("agent.artifact.create", {
          taskId: context.taskId, runId: context.runId,
          attributes: { "dipole.agent.artifact.type": "conversation_digest", "dipole.agent.artifact.version": 1 }
        }, async artifactSpan => {
          const value = await dependencies.artifacts!.createArtifact({
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
          artifactSpan.setAttribute("dipole.agent.artifact.size_bytes", value.sizeBytes);
          return value;
        });
        span.setAttribute("dipole.agent.run.step_count", plan.steps.length);
        return {
          kind: "complete",
          output: {
            summary: plan.summary,
            stepCount: plan.steps.length,
            ...(artifact === undefined ? {} : { artifactId: artifact.artifactId, artifactVersion: artifact.version })
          }
        };
      });
    }
  };
}
