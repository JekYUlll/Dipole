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
export const discoveredConversationMarker = "$discovered.previous";

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
  synthesize?(event: AgentEvent, context: ExecutionContext, plan: ShadowPlan, outputs: readonly unknown[]): Promise<string>;
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

export const maxReadScopeOptions = 8;
const maxReadScopeOptionLength = 128;

export interface ShadowReadScope {
  /** Pause for owner confirmation instead of auto-selecting when discovery offers several conversations. */
  readonly confirmationRequired?: boolean;
  /** Conversation key the Task owner confirmed. Callers must bind it to the offered candidates. */
  readonly confirmedConversationId?: string;
  /** Host-owned plan of a resumed Run. Present plans are neither re-planned nor re-audited. */
  readonly resumedPlan?: ShadowPlan;
}

export type ShadowPlanExecution =
  | { readonly outcome: "completed"; readonly plan: ShadowPlan }
  | {
    readonly outcome: "awaiting_read_scope";
    readonly plan: ShadowPlan;
    readonly stepNo: number;
    readonly candidates: readonly string[];
    readonly discoveredCount: number;
  };

export function agentTaskId(input: { tenantId: string; agentUuid: string; triggerType: string; triggerRef: string; subscriptionId?: string }): string {
  const canonical = [
    policyVersion,
    input.tenantId.trim(),
    input.agentUuid.trim(),
    input.triggerType.trim(),
    input.triggerRef.trim(),
    ...(input.subscriptionId?.trim() ? [input.subscriptionId.trim()] : [])
  ].join("\n");
  return `task:${createHash("sha256").update(canonical, "utf8").digest("hex").slice(0, 59)}`;
}

export function agentEventLedgerKey(event: Pick<AgentEvent, "eventId" | "subscriptionId">): string {
  if (event.subscriptionId === undefined) return event.eventId;
  const canonical = [event.eventId.trim(), event.subscriptionId.trim()].join("\n");
  return `subscription:${createHash("sha256").update(canonical, "utf8").digest("hex").slice(0, 64)}`;
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
      triggerRef: event.aggregateId,
      ...(event.subscriptionId === undefined ? {} : { subscriptionId: event.subscriptionId })
    });
    return this.telemetry.withSpan("agent.task", {
      taskId,
      attributes: { "dipole.agent.event.type": event.eventType, "dipole.agent.mode": "shadow" }
    }, async taskSpan => {
      if (event.lineage?.origin.type === "agent" && event.lineage.origin.id === identity.agentUuid.trim()) {
        taskSpan.setAttribute("dipole.agent.task.outcome", "suppressed");
        return { outcome: "suppressed", taskId };
      }
      const claim = await this.ledger.claim(agentEventLedgerKey(event), taskId, event.eventType);
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
  dependencies: ShadowPlanExecutionDependencies,
  scope?: ShadowReadScope
): Promise<ShadowPlanExecution> {
  const plan = scope?.resumedPlan ?? await dependencies.planner.plan(event, context);
  if (scope?.resumedPlan === undefined) {
    await dependencies.audit.append({ eventId: event.eventId, taskId: context.taskId, eventType: event.eventType, plan });
  }
  const execution = await executeShadowPlanSteps(
    plan, context, dependencies.registry, dependencies.trajectory,
    dependencies.stepLeaseMs, dependencies.busyStepRetry, dependencies.telemetry ?? new AgentTelemetry(), scope
  );
  if (execution.outcome === "awaiting_read_scope") {
    return { ...execution, plan };
  }
  const summary = dependencies.planner.synthesize === undefined
    ? plan.summary
    : await dependencies.planner.synthesize(event, context, plan, execution.outputs);
  return { outcome: "completed", plan: summary === plan.summary ? plan : { ...plan, summary } };
}

async function executeShadowPlanSteps(
  plan: ShadowPlan,
  context: ExecutionContext,
  registry: CapabilityRegistry,
  trajectory: ShadowStepTrajectory,
  stepLeaseMs: number,
  busyStepRetry?: ShadowPlanExecutionDependencies["busyStepRetry"],
  telemetry: Pick<AgentTelemetry, "withSpan"> = new AgentTelemetry(),
  scope?: ShadowReadScope
): Promise<
  { readonly outcome: "completed"; readonly outputs: readonly unknown[] } |
  { readonly outcome: "awaiting_read_scope"; readonly stepNo: number; readonly candidates: readonly string[]; readonly discoveredCount: number }
> {
  const outputs: unknown[] = [];
  for (const [index, step] of plan.steps.entries()) {
    const stepNo = index + 1;
    // Decide before claiming so the paused read Step stays unclaimed for the confirmed resume.
    if (scope?.confirmationRequired === true && scope.confirmedConversationId === undefined &&
        isDiscoveryBoundRead(plan.steps, index, step)) {
      const discovered = discoveredConversationIds(outputs.at(-1));
      const candidates = readScopeOptions(discovered);
      if (candidates.length > 1) {
        if (plan.steps.length !== 2 || index !== 1) {
          throw new Error("Owner-confirmed read scope supports a single conversation.list to conversation.read pair");
        }
        return { outcome: "awaiting_read_scope", stepNo, candidates, discoveredCount: discovered.length };
      }
    }
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
      const resolvedStep = resolveTrustedDiscoveryStep(plan.steps, index, step, outputs, scope);
      if (resolvedStep === undefined) {
        const output = { status: "skipped", reason: "no_discovered_conversation" };
        await trajectory.completeStep(context.taskId, stepNo, claimToken, output);
        outputs.push(output);
        continue;
      }
      const invocation = registry.prepare(resolvedStep.capabilityId, resolvedStep.input, context);
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
      outputs.push(output);
    } catch (error) {
      await trajectory.failStep(context.taskId, stepNo, claimToken, error);
      throw error;
    }
  }
  return { outcome: "completed", outputs };
}

function resolveTrustedDiscoveryStep(
  steps: readonly ShadowPlanStep[],
  index: number,
  step: ShadowPlanStep,
  outputs: readonly unknown[],
  scope?: ShadowReadScope
): ShadowPlanStep | undefined {
  if (step.capabilityId !== "conversation.read") return step;
  if (step.input.conversationId !== discoveredConversationMarker) {
    throw new Error("conversation.read requires the trusted conversation discovery marker");
  }
  if (index === 0 || steps[index - 1]?.capabilityId !== "conversation.list") {
    throw new Error("conversation.read requires the immediately preceding trusted conversation.list result");
  }
  // A confirmed scope replaces the discovery output of a resumed Run, whose
  // completed list Step no longer replays its own result. Core still authorises
  // the read, and the caller binds the confirmation to the offered candidates.
  if (scope?.confirmedConversationId !== undefined) {
    return { ...step, input: { ...step.input, conversationId: scope.confirmedConversationId } };
  }
  const conversationId = discoveredConversationIds(outputs.at(-1))[0];
  // An empty list is a valid user state. Keep the skipped result in the
  // trajectory, but never turn it into a guessed or unauthorised read.
  if (conversationId === undefined) return undefined;
  return { ...step, input: { ...step.input, conversationId } };
}

function isDiscoveryBoundRead(steps: readonly ShadowPlanStep[], index: number, step: ShadowPlanStep): boolean {
  return step.capabilityId === "conversation.read" &&
    step.input.conversationId === discoveredConversationMarker &&
    index > 0 && steps[index - 1]?.capabilityId === "conversation.list";
}

function discoveredConversationIds(output: unknown): readonly string[] {
  if (!Array.isArray(output)) return [];
  const conversationIds: string[] = [];
  for (const item of output) {
    if (item === null || typeof item !== "object") continue;
    const value = (item as { conversationKey?: unknown }).conversationKey;
    if (typeof value !== "string") continue;
    const conversationId = value.trim();
    if (conversationId.length === 0 || conversationId.length > 256 || conversationIds.includes(conversationId)) continue;
    conversationIds.push(conversationId);
  }
  return conversationIds;
}

function readScopeOptions(discovered: readonly string[]): readonly string[] {
  return discovered.filter((conversationId) => conversationId.length <= maxReadScopeOptionLength).slice(0, maxReadScopeOptions);
}

function delay(milliseconds: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}
