import { createHash } from "node:crypto";

import { z } from "zod";

import { executionContextSchema, type ExecutionContext } from "../runtime/execution-context.js";
import type { EventLedger } from "./event-ledger.js";

const policyVersion = "dipole.agent.policy.persistence.v1";

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
  readonly capabilityIds: readonly string[];
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

export type ShadowProcessResult = { readonly outcome: "recorded" | "duplicate"; readonly taskId: string };

export function agentTaskId(input: { tenantId: string; agentUuid: string; triggerType: string; triggerRef: string }): string {
  const canonical = [policyVersion, input.tenantId.trim(), input.agentUuid.trim(), input.triggerType.trim(), input.triggerRef.trim()].join("\n");
  return `task:${createHash("sha256").update(canonical, "utf8").digest("hex").slice(0, 59)}`;
}

export class ShadowEventProcessor {
  constructor(private readonly planner: ShadowPlanner, private readonly audit: ShadowAuditSink, private readonly ledger: EventLedger) {}

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
      const context = executionContextSchema.parse({
        tenantId: identity.tenantId,
        principalUuid: identity.principalUuid,
        agentUuid: identity.agentUuid,
        taskId,
        mode: "shadow",
        permissions: ["conversation.read"],
        resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["read", "list"] }],
        approvedCapabilities: [],
        eventId: event.eventId,
        ...(identity.requestId === undefined ? {} : { requestId: identity.requestId }),
        ...(identity.traceId === undefined ? {} : { traceId: identity.traceId })
      });
      const plan = await this.planner.plan(event, context);
      await this.audit.append({ eventId: event.eventId, taskId, eventType: event.eventType, plan });
      await this.ledger.complete(claim);
      return { outcome: "recorded", taskId };
    } catch (error) {
      await this.ledger.release(claim, error);
      throw error;
    }
  }
}
