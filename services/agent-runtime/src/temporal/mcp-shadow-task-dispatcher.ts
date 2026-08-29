import { z } from "zod";

import {
  agentEventSchema,
  agentTaskId,
  type AgentEvent,
  type AgentIdentity,
  type ShadowTaskDispatcher
} from "../events/shadow-processor.js";
import type { TemporalMcpTaskClient } from "./temporal-task-client.js";

const identifier = (maximum: number) => z.string().trim().min(1).max(maximum);
const identitySchema = z.object({
  tenantId: identifier(64),
  principalUuid: identifier(128),
  agentUuid: identifier(128),
  requestId: identifier(128).optional(),
  traceId: identifier(128).optional()
}).strict();
const routeSelectionSchema = z.object({
  routeId: identifier(128).regex(/^[A-Za-z0-9][A-Za-z0-9_.:-]*$/),
  arguments: z.record(z.string(), z.unknown())
}).strict();

export interface TemporalMcpShadowRouteSelection {
  readonly routeId: string;
  readonly arguments: Readonly<Record<string, unknown>>;
}

export interface TemporalMcpShadowRouteSelector {
  select(
    event: AgentEvent,
    identity: AgentIdentity
  ): TemporalMcpShadowRouteSelection | Promise<TemporalMcpShadowRouteSelection>;
}

export class TemporalMcpShadowTaskDispatcher implements ShadowTaskDispatcher {
  constructor(
    private readonly tasks: Pick<TemporalMcpTaskClient, "start">,
    private readonly routes: TemporalMcpShadowRouteSelector
  ) {}

  async dispatch(rawEvent: AgentEvent, rawIdentity: AgentIdentity, rawTaskId: string): Promise<void> {
    const event = Object.freeze(agentEventSchema.parse(rawEvent));
    const parsedIdentity = identitySchema.parse(rawIdentity);
    const identity: AgentIdentity = Object.freeze({
      tenantId: parsedIdentity.tenantId,
      principalUuid: parsedIdentity.principalUuid,
      agentUuid: parsedIdentity.agentUuid,
      ...(parsedIdentity.requestId === undefined ? {} : { requestId: parsedIdentity.requestId }),
      ...(parsedIdentity.traceId === undefined ? {} : { traceId: parsedIdentity.traceId })
    });
    const expectedTaskId = agentTaskId({
      tenantId: identity.tenantId,
      agentUuid: identity.agentUuid,
      triggerType: event.eventType,
      triggerRef: event.aggregateId
    });
    if (rawTaskId !== expectedTaskId) {
      throw new Error("Temporal MCP Shadow Task ID does not match the deterministic event binding");
    }

    const admission = Object.freeze({
      tenantId: identity.tenantId,
      principalUserId: identity.principalUuid,
      agentId: identity.agentUuid,
      triggerType: event.eventType,
      triggerRef: event.aggregateId,
      eventId: event.eventId,
      ...(event.subscriptionId === undefined ? {} : { subscriptionId: event.subscriptionId }),
      ...(identity.requestId === undefined ? {} : { requestId: identity.requestId }),
      ...(identity.traceId === undefined ? {} : { traceId: identity.traceId })
    });

    const rawSelection = await this.routes.select(event, identity);
    const parsedSelection = routeSelectionSchema.safeParse(rawSelection);
    if (!parsedSelection.success) {
      throw new Error("Temporal MCP Shadow route selection is invalid");
    }

    await this.tasks.start({
      taskId: expectedTaskId,
      goal: `execute external MCP route ${parsedSelection.data.routeId}`,
      routeId: parsedSelection.data.routeId,
      arguments: parsedSelection.data.arguments,
      admission
    });
  }
}
