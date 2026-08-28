import { z } from "zod";

import { agentEventSchema, type AgentEvent, type AgentIdentity } from "../events/shadow-processor.js";
import type {
  TemporalMcpShadowRouteSelection,
  TemporalMcpShadowRouteSelector
} from "./mcp-shadow-task-dispatcher.js";

const routeDefinitionSchema = z.object({
  definitionId: z.string().trim().min(1).max(64),
  definitionVersion: z.number().int().positive(),
  routeId: z.string().trim().min(1).max(128).regex(/^[A-Za-z0-9][A-Za-z0-9_.:-]*$/),
  resolveArguments: z.custom<TemporalMcpSubscriptionRouteDefinition["resolveArguments"]>(
    (value) => typeof value === "function"
  )
}).strict();
const argumentsSchema = z.record(z.string(), z.unknown());

export interface TemporalMcpSubscriptionRouteDefinition {
  readonly definitionId: string;
  readonly definitionVersion: number;
  readonly routeId: string;
  resolveArguments(event: AgentEvent, identity: AgentIdentity): unknown | Promise<unknown>;
}

interface RegisteredRoute {
  readonly routeId: string;
  resolveArguments(event: AgentEvent, identity: AgentIdentity): unknown | Promise<unknown>;
}

export class TemporalMcpSubscriptionRouteSelector implements TemporalMcpShadowRouteSelector {
  readonly #routes = new Map<string, RegisteredRoute>();

  constructor(rawDefinitions: readonly TemporalMcpSubscriptionRouteDefinition[]) {
    if (rawDefinitions.length === 0) {
      throw new Error("Temporal MCP subscription routes are unavailable");
    }
    for (const rawDefinition of rawDefinitions) {
      const parsed = routeDefinitionSchema.safeParse(rawDefinition);
      if (!parsed.success) {
        throw new Error("Temporal MCP subscription route definition is invalid");
      }
      const key = definitionKey(parsed.data.definitionId, parsed.data.definitionVersion);
      if (this.#routes.has(key)) {
        throw new Error("Temporal MCP subscription definition binding is duplicated");
      }
      this.#routes.set(key, Object.freeze({
        routeId: parsed.data.routeId,
        resolveArguments: parsed.data.resolveArguments
      }));
    }
  }

  async select(rawEvent: AgentEvent, identity: AgentIdentity): Promise<TemporalMcpShadowRouteSelection> {
    const parsedEvent = agentEventSchema.safeParse(rawEvent);
    if (!parsedEvent.success) {
      throw new Error("Temporal MCP subscription binding is invalid");
    }
    const event = Object.freeze(parsedEvent.data);
    const binding = event.subscriptionBinding;
    if (binding === undefined) {
      throw new Error("Temporal MCP subscription binding is unavailable");
    }
    if (binding.tenantId !== identity.tenantId.trim() || binding.agentId !== identity.agentUuid.trim()) {
      throw new Error("Temporal MCP subscription binding is invalid");
    }
    const route = this.#routes.get(definitionKey(binding.definitionId, binding.definitionVersion));
    if (route === undefined) {
      throw new Error("Temporal MCP subscription route is unavailable");
    }
    const parsedArguments = argumentsSchema.safeParse(await route.resolveArguments(event, identity));
    if (!parsedArguments.success) {
      throw new Error("Temporal MCP subscription route arguments are invalid");
    }
    return Object.freeze({ routeId: route.routeId, arguments: Object.freeze(parsedArguments.data) });
  }
}

function definitionKey(definitionId: string, definitionVersion: number): string {
  return `${definitionId}\u0000${definitionVersion}`;
}
