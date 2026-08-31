import { createHash } from "node:crypto";

import { z } from "zod";

import { executionContextSchema, type ExecutionContext } from "../runtime/execution-context.js";
import type { CapabilityRegistry } from "../capabilities/registry.js";
import type { EventLedger } from "./event-ledger.js";
import { AgentTelemetry } from "../observability/agent-telemetry.js";

const policyVersion = "dipole.agent.policy.persistence.v1";
const runIDVersion = "dipole.agent.run.v1";
const lineageIdentifier = /^[A-Za-z0-9][A-Za-z0-9._:/-]*$/;
const defaultReadPermissions = ["conversation.list", "conversation.read"] as const;

const eventLineageSchema = z.object({
  origin: z.object({
    type: z.enum(["agent", "service", "system"]),
    id: z.string().trim().min(1).max(128).regex(lineageIdentifier)
  }).strict(),
  causationEventId: z.string().trim().min(1).max(128).regex(lineageIdentifier).optional(),
  agentTaskId: z.string().trim().min(1).max(128).regex(lineageIdentifier).optional()
}).strict().superRefine((lineage, context) => {
  if (lineage.origin.type === "agent" && lineage.agentTaskId === undefined) {
    context.addIssue({ code: "custom", path: ["agentTaskId"], message: "agentTaskId is required for Agent origin" });
  }
});

const subscriptionBindingSchema = z.object({
  subscriptionId: z.string().trim().min(1).max(64),
  definitionId: z.string().trim().min(1).max(64),
  definitionVersion: z.number().int().positive(),
  tenantId: z.string().trim().min(1).max(64),
  agentId: z.string().trim().min(1).max(24)
}).strict();

export const agentEventSchema = z.object({
  eventId: z.string().trim().min(1),
  eventType: z.string().trim().min(1),
  aggregateId: z.string().trim().min(1),
  occurredAt: z.iso.datetime(),
  payload: z.record(z.string(), z.unknown()),
  lineage: eventLineageSchema.optional(),
  subscriptionId: z.string().trim().min(1).max(64).optional(),
  subscriptionBinding: subscriptionBindingSchema.optional()
}).strict().superRefine((event, context) => {
  if (event.subscriptionBinding !== undefined && event.subscriptionBinding.subscriptionId !== event.subscriptionId) {
    context.addIssue({
      code: "custom",
      path: ["subscriptionBinding", "subscriptionId"],
      message: "subscription binding does not match the event subscription"
    });
  }
});

export type AgentEvent = z.infer<typeof agentEventSchema>;

export interface AgentIdentity {
  readonly tenantId: string;
  readonly principalUuid: string;
  readonly agentUuid: string;
  readonly requestId?: string;
  readonly traceId?: string;
}

export interface ShadowPlan {
  readonly summary: string;
  readonly steps: readonly ShadowPlanStep[];
  readonly model?: {
    readonly route: string;
    readonly attempts: number;
    readonly inputTokens: number | undefined;
    readonly outputTokens: number | undefined;
    readonly context?: {
      readonly compilerVersion: "v1" | "v2";
      readonly estimatorId?: string;
      readonly estimatedTokens: number;
      readonly selected: readonly {
        readonly id: string;
        readonly representation: "full" | "compact";
        readonly provenance: {
          readonly sourceType: string;
          readonly sourceId: string;
          readonly uri?: string;
          readonly sequence?: string;
        };
        readonly contentSha256?: string;
      }[];
      readonly omitted: readonly string[];
    };
  };
}

export interface ShadowPlanStep {
  readonly capabilityId: string;
  readonly input: Readonly<Record<string, unknown>>;
}

export interface ShadowPlanner {
  plan(event: AgentEvent, context: ExecutionContext): Promise<ShadowPlan>;
}

export interface ShadowAuditRecord {
  readonly eventId: string;
  readonly taskId: string;
  readonly eventType: string;
  readonly plan: ShadowPlan;
}

export interface ShadowAuditSink {
  append(record: ShadowAuditRecord): Promise<void>;
}

export interface ShadowRunAdmission {
  admit(event: AgentEvent, identity: AgentIdentity): Promise<{
    readonly taskId: string;
    readonly runId: string;
    readonly runStatus: "running" | "completed";
  }>;
  complete(taskId: string, runId: string, context?: Pick<ExecutionContext, "requestId" | "traceId">): Promise<void>;
}

export interface ShadowTaskDispatcher {
  dispatch(event: AgentEvent, identity: AgentIdentity, taskId: string): Promise<void>;
}

export interface ShadowStepTrajectory extends ShadowAuditSink {
  claimStep(taskId: string, stepNo: number, leaseMs: number): Promise<
    { readonly outcome: "claimed"; readonly token: string } | { readonly outcome: "completed" | "busy" }
  >;
  recordAuthorization?(taskId: string, stepNo: number, token: string, resource: {
    readonly resourceType: string; readonly resourceId: string; readonly action: string;
  }, decision: "allowed"): Promise<void>;
  completeStep(taskId: string, stepNo: number, token: string, output: unknown): Promise<void>;
  failStep(taskId: string, stepNo: number, token: string, error: unknown): Promise<void>;
}

export interface ShadowPlanExecutionDependencies {
  readonly planner: ShadowPlanner;
  readonly audit: ShadowAuditSink;
  readonly registry: CapabilityRegistry;
  readonly trajectory: ShadowStepTrajectory;
  readonly stepLeaseMs: number;
  readonly busyStepRetry?: {
    readonly intervalMs: number;
    readonly maxWaitMs: number;
  };
  readonly telemetry?: Pick<AgentTelemetry, "withSpan">;
}

export type ShadowProcessResult = { readonly outcome: "recorded" | "duplicate" | "suppressed"; readonly taskId: string };

export function agentTaskId(input: { tenantId: string; agentUuid: string; triggerType: string; triggerRef: string }): string {
  const canonical = [policyVersion, input.tenantId.trim(), input.agentUuid.trim(), input.triggerType.trim(), input.triggerRef.trim()].join("\n");
  return `task:${createHash("sha256").update(canonical, "utf8").digest("hex").slice(0, 59)}`;
}

export function agentRunId(taskId: string, runtimeId = "dipole-agent", mode = "shadow"): string {
  const canonical = [runIDVersion, taskId.trim(), runtimeId.trim(), mode.trim()].join("\n");
  return `run:${createHash("sha256").update(canonical, "utf8").digest("hex").slice(0, 60)}`;
}

export class ShadowEventProcessor {
  constructor(
    private readonly planner: ShadowPlanner,
    private readonly audit: ShadowAuditSink,
    private readonly ledger: EventLedger,
    private readonly admission?: ShadowRunAdmission,
    private readonly registry?: CapabilityRegistry,
    private readonly trajectory?: ShadowStepTrajectory,
    private readonly stepLeaseMs = 60_000,
    private readonly dispatcher?: ShadowTaskDispatcher,
    private readonly telemetry: Pick<AgentTelemetry, "withSpan"> = new AgentTelemetry(),
    private readonly readPermissions: readonly string[] = defaultReadPermissions
  ) {
    if ((registry === undefined) !== (trajectory === undefined)) {
      throw new Error("Capability Registry and Step trajectory must be configured together");
    }
  }

  async process(rawEvent: unknown, identity: AgentIdentity): Promise<ShadowProcessResult> {
    const event = agentEventSchema.parse(rawEvent);
    const taskId = agentTaskId({
      tenantId: identity.tenantId,
      agentUuid: identity.agentUuid,
      triggerType: event.eventType,
      triggerRef: event.aggregateId
    });
    return this.telemetry.withSpan("agent.task", {
      taskId,
      attributes: { "dipole.agent.event.type": event.eventType, "dipole.agent.mode": "shadow" }
    }, async taskSpan => {
      if (event.lineage?.origin.type === "agent" && event.lineage.origin.id === identity.agentUuid.trim()) {
        taskSpan.setAttribute("dipole.agent.task.outcome", "suppressed");
        return { outcome: "suppressed", taskId };
      }
      const claim = await this.ledger.claim(event.eventId, taskId, event.eventType);
      if (claim === undefined) {
        taskSpan.setAttribute("dipole.agent.task.outcome", "duplicate");
        return { outcome: "duplicate", taskId };
      }
      try {
      if (this.dispatcher !== undefined) {
        await this.dispatcher.dispatch(event, identity, taskId);
        await this.ledger.complete(claim);
        taskSpan.setAttribute("dipole.agent.task.outcome", "recorded");
        return { outcome: "recorded", taskId };
      }
      const expectedRunId = agentRunId(taskId);
      const admitted = this.admission === undefined
        ? { taskId, runId: expectedRunId, runStatus: "running" as const }
        : await this.admission.admit(event, identity);
      if (admitted.taskId !== taskId || admitted.runId !== expectedRunId) {
        throw new Error("Agent Run admission identity does not match the deterministic event binding");
      }
      if (admitted.runStatus === "completed") {
        await this.ledger.complete(claim);
        return { outcome: "recorded", taskId };
      }
      const context = executionContextSchema.parse({
        tenantId: identity.tenantId,
        principalUuid: identity.principalUuid,
        agentUuid: identity.agentUuid,
        taskId,
        runId: admitted.runId,
        mode: "shadow",
        permissions: this.readPermissions,
        resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["read", "list"] }],
        approvedCapabilities: [],
        eventId: event.eventId,
        ...(identity.requestId === undefined ? {} : { requestId: identity.requestId }),
        ...(identity.traceId === undefined ? {} : { traceId: identity.traceId })
      });
      return await this.telemetry.withSpan("agent.run", {
        taskId, runId: admitted.runId, attributes: { "dipole.agent.mode": context.mode }
      }, async runSpan => {
        const plan = await this.planner.plan(event, context);
        await this.audit.append({ eventId: event.eventId, taskId, eventType: event.eventType, plan });
        if (this.registry !== undefined && this.trajectory !== undefined) {
          await executeShadowPlanSteps(plan, context, this.registry, this.trajectory, this.stepLeaseMs, undefined, this.telemetry);
        }
        await this.admission?.complete(taskId, admitted.runId, context);
        await this.ledger.complete(claim);
        runSpan.setAttribute("dipole.agent.run.step_count", plan.steps.length);
        taskSpan.setAttribute("dipole.agent.task.outcome", "recorded");
        return { outcome: "recorded", taskId };
      });
      } catch (error) {
        await this.ledger.release(claim, error);
        throw error;
      }
    });
  }

}

export async function executeShadowPlan(
  event: AgentEvent,
  context: ExecutionContext,
  dependencies: ShadowPlanExecutionDependencies
): Promise<ShadowPlan> {
  const plan = await dependencies.planner.plan(event, context);
  await dependencies.audit.append({ eventId: event.eventId, taskId: context.taskId, eventType: event.eventType, plan });
  await executeShadowPlanSteps(
    plan, context, dependencies.registry, dependencies.trajectory,
    dependencies.stepLeaseMs, dependencies.busyStepRetry, dependencies.telemetry ?? new AgentTelemetry()
  );
  return plan;
}

async function executeShadowPlanSteps(
  plan: ShadowPlan,
  context: ExecutionContext,
  registry: CapabilityRegistry,
  trajectory: ShadowStepTrajectory,
  stepLeaseMs: number,
  busyStepRetry?: ShadowPlanExecutionDependencies["busyStepRetry"],
  telemetry: Pick<AgentTelemetry, "withSpan"> = new AgentTelemetry()
): Promise<void> {
  for (const [index, step] of plan.steps.entries()) {
    const stepNo = index + 1;
    let claim = await trajectory.claimStep(context.taskId, stepNo, stepLeaseMs);
    let waitedMs = 0;
    while (claim.outcome === "busy" && busyStepRetry !== undefined && waitedMs < busyStepRetry.maxWaitMs) {
      const delayMs = Math.min(busyStepRetry.intervalMs, busyStepRetry.maxWaitMs - waitedMs);
      await delay(delayMs);
      waitedMs += delayMs;
      claim = await trajectory.claimStep(context.taskId, stepNo, stepLeaseMs);
    }
    if (claim.outcome !== "claimed") {
      if (claim.outcome === "completed") continue;
      throw new Error(`Agent Step ${stepNo} is owned by another worker`);
    }
    const claimToken = claim.token;
    try {
      const invocation = registry.prepare(step.capabilityId, step.input, context);
      if (trajectory.recordAuthorization === undefined) throw new Error("Agent shadow Step authorization audit is unavailable");
      await trajectory.recordAuthorization(context.taskId, stepNo, claimToken, invocation.resource, "allowed");
      const output = await telemetry.withSpan("agent.tool.call", {
        taskId: context.taskId, runId: context.runId,
        attributes: {
          "dipole.agent.capability.id": step.capabilityId,
          "dipole.agent.step.number": stepNo,
          "dipole.agent.tool.transport": "native"
        }
      }, async () => invocation.execute());
      await trajectory.completeStep(context.taskId, stepNo, claimToken, output);
    } catch (error) {
      await trajectory.failStep(context.taskId, stepNo, claimToken, error);
      throw error;
    }
  }
}

function delay(milliseconds: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}
