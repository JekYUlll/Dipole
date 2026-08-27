import { createHash } from "node:crypto";

import { z } from "zod";

import { executionContextSchema, type ExecutionContext } from "../runtime/execution-context.js";
import type { CapabilityRegistry } from "../capabilities/registry.js";
import type { EventLedger } from "./event-ledger.js";

const policyVersion = "dipole.agent.policy.persistence.v1";
const runIDVersion = "dipole.agent.run.v1";

export const agentEventSchema = z.object({
  eventId: z.string().trim().min(1),
  eventType: z.string().trim().min(1),
  aggregateId: z.string().trim().min(1),
  occurredAt: z.iso.datetime(),
  payload: z.record(z.string(), z.unknown())
}).strict();

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
      readonly compilerVersion: "v1";
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

export interface ShadowStepTrajectory extends ShadowAuditSink {
  claimStep(taskId: string, stepNo: number, leaseMs: number): Promise<
    { readonly outcome: "claimed"; readonly token: string } | { readonly outcome: "completed" | "busy" }
  >;
  completeStep(taskId: string, stepNo: number, token: string, output: unknown): Promise<void>;
  failStep(taskId: string, stepNo: number, token: string, error: unknown): Promise<void>;
}

export type ShadowProcessResult = { readonly outcome: "recorded" | "duplicate"; readonly taskId: string };

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
    private readonly stepLeaseMs = 60_000
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
    const claim = await this.ledger.claim(event.eventId, taskId, event.eventType);
    if (claim === undefined) {
      return { outcome: "duplicate", taskId };
    }
    try {
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
        permissions: ["conversation.list", "conversation.read"],
        resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["read", "list"] }],
        approvedCapabilities: [],
        eventId: event.eventId,
        ...(identity.requestId === undefined ? {} : { requestId: identity.requestId }),
        ...(identity.traceId === undefined ? {} : { traceId: identity.traceId })
      });
      const plan = await this.planner.plan(event, context);
      await this.audit.append({ eventId: event.eventId, taskId, eventType: event.eventType, plan });
      if (this.registry !== undefined && this.trajectory !== undefined) {
        await this.executeSteps(plan, context);
      }
      await this.admission?.complete(taskId, admitted.runId, context);
      await this.ledger.complete(claim);
      return { outcome: "recorded", taskId };
    } catch (error) {
      await this.ledger.release(claim, error);
      throw error;
    }
  }

  private async executeSteps(plan: ShadowPlan, context: ExecutionContext): Promise<void> {
    for (const [index, step] of plan.steps.entries()) {
      const stepNo = index + 1;
      const claim = await this.trajectory!.claimStep(context.taskId, stepNo, this.stepLeaseMs);
      if (claim.outcome !== "claimed") {
        if (claim.outcome === "completed") continue;
        throw new Error(`Agent Step ${stepNo} is owned by another worker`);
      }
      const claimToken = claim.token;
      try {
        const output = await this.registry!.execute(step.capabilityId, step.input, context);
        await this.trajectory!.completeStep(context.taskId, stepNo, claimToken, output);
      } catch (error) {
        await this.trajectory!.failStep(context.taskId, stepNo, claimToken, error);
        throw error;
      }
    }
  }
}
