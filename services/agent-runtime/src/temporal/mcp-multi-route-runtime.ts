import { z } from "zod";

import type { McpWorkerExternalMcpDependencies } from "../mcp/mcp-worker-runtime.js";
import type { ExternalMcpCapabilityRouteRegistry } from "../mcp/mcp-invocation-producer.js";
import {
  temporalMcpDispatchRouteBinding,
  type TemporalMcpDispatchActivities,
  type TemporalMcpDispatchActivityInput,
  type TemporalMcpDispatchRoute,
  type TemporalMcpDispatchRouteBinding
} from "./mcp-dispatch-activity.js";
import {
  createTemporalMcpDispatchRuntime,
  type TemporalMcpDispatchRuntime,
  type TemporalMcpDispatchRuntimeDependencies
} from "./mcp-dispatch-runtime.js";

const routeIdSchema = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/);
const routingInputSchema = z.discriminatedUnion("kind", [
  z.object({ kind: z.literal("begin"), routeId: routeIdSchema }).passthrough(),
  z.object({
    kind: z.literal("resume"),
    checkpoint: z.object({ routeId: routeIdSchema }).passthrough()
  }).passthrough()
]);

export interface TemporalMcpMultiRoutePlan {
  readonly routes: readonly TemporalMcpDispatchRoute[];
  readonly routeRegistry: ExternalMcpCapabilityRouteRegistry;
  readonly workerExternalMcp: McpWorkerExternalMcpDependencies;
}

export type TemporalMcpMultiRouteRuntimeDependencies = Omit<
  TemporalMcpDispatchRuntimeDependencies,
  "routes" | "externalMcp"
>;

export type TemporalMcpDispatchRuntimeFactory = (
  route: TemporalMcpDispatchRoute,
  dependencies: TemporalMcpDispatchRuntimeDependencies
) => TemporalMcpDispatchRuntime;

export interface TemporalMcpMultiRouteRuntime {
  readonly routeBindings: readonly TemporalMcpDispatchRouteBinding[];
  readonly activities: TemporalMcpDispatchActivities;
}

export function createTemporalMcpMultiRouteRuntime(
  plan: TemporalMcpMultiRoutePlan,
  dependencies: TemporalMcpMultiRouteRuntimeDependencies,
  createRuntime: TemporalMcpDispatchRuntimeFactory = createTemporalMcpDispatchRuntime
): TemporalMcpMultiRouteRuntime {
  if (plan.routes.length === 0) throw new Error("Temporal MCP deployment routes are unavailable");

  const routeBindings = plan.routes.map(temporalMcpDispatchRouteBinding);
  const routeIds = new Set<string>();
  for (const binding of routeBindings) {
    if (routeIds.has(binding.routeId)) throw new Error("Temporal MCP deployment route ID is duplicated");
    routeIds.add(binding.routeId);
  }

  const runtimes = new Map<string, TemporalMcpDispatchRuntime>();
  for (const route of plan.routes) {
    runtimes.set(route.routeId, createRuntime(route, {
      ...dependencies,
      routes: plan.routeRegistry,
      externalMcp: plan.workerExternalMcp
    }));
  }

  return {
    routeBindings: Object.freeze([...routeBindings]),
    activities: {
      executeMcpDispatch: input => dispatch(runtimes, input)
    }
  };
}

async function dispatch(
  runtimes: ReadonlyMap<string, TemporalMcpDispatchRuntime>,
  input: TemporalMcpDispatchActivityInput
) {
  const parsed = routingInputSchema.safeParse(input);
  if (!parsed.success) throw new Error("Temporal MCP dispatch routing input is invalid");
  const routeId = parsed.data.kind === "begin" ? parsed.data.routeId : parsed.data.checkpoint.routeId;
  const runtime = runtimes.get(routeId);
  if (runtime === undefined) throw new Error("Temporal MCP dispatch route is unavailable");
  return runtime.activities.executeMcpDispatch(input);
}
