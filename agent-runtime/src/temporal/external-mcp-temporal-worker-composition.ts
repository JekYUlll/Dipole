import type { AgentTaskWorkerActivities } from "./agent-task-activities.js";
import {
  temporalMcpDispatchRouteBinding,
  type TemporalMcpDispatchActivities,
  type TemporalMcpDispatchRouteBinding
} from "./mcp-dispatch-activity.js";
import {
  createTemporalMcpMultiRouteRuntime,
  type TemporalMcpMultiRoutePlan,
  type TemporalMcpMultiRouteRuntime,
  type TemporalMcpMultiRouteRuntimeDependencies
} from "./mcp-multi-route-runtime.js";
import { TemporalMcpWorkflowExecutionCatalog } from "./mcp-workflow-envelope.js";
import { externalMcpReadinessBindingSha256 } from "../mcp/external-mcp-readiness-evidence.js";
import {
  TemporalMcpSubscriptionRouteSelector,
  type TemporalMcpSubscriptionRouteDefinition
} from "./mcp-subscription-route-selector.js";

const sha256Pattern = /^[a-f0-9]{64}$/;

export interface ExternalMcpTemporalWorkerCompositionPlan extends TemporalMcpMultiRoutePlan {
  readonly runtimeBindingSha256: string;
  readonly subscriptionRoutes?: readonly TemporalMcpSubscriptionRouteDefinition[];
}

export type ExternalMcpTemporalWorkerActivities = AgentTaskWorkerActivities & TemporalMcpDispatchActivities;

export type TemporalMcpMultiRouteRuntimeFactory = (
  plan: TemporalMcpMultiRoutePlan,
  dependencies: TemporalMcpMultiRouteRuntimeDependencies
) => TemporalMcpMultiRouteRuntime;

export interface ExternalMcpTemporalWorkerComposition {
  readonly activities: ExternalMcpTemporalWorkerActivities;
  readonly routeBindings: readonly TemporalMcpDispatchRouteBinding[];
  readonly workflowExecutions: TemporalMcpWorkflowExecutionCatalog;
  readonly runtimeBindingSha256: string;
  readonly subscriptionRoutes: readonly TemporalMcpSubscriptionRouteDefinition[];
}

export function createExternalMcpTemporalWorkerComposition(
  plan: ExternalMcpTemporalWorkerCompositionPlan | undefined,
  baseActivities: AgentTaskWorkerActivities,
  resolveDependencies: () => TemporalMcpMultiRouteRuntimeDependencies,
  createRuntime: TemporalMcpMultiRouteRuntimeFactory = createTemporalMcpMultiRouteRuntime
): ExternalMcpTemporalWorkerComposition | undefined {
  if (plan === undefined) return undefined;
  validateExternalMcpTemporalWorkerCompositionPlan(plan, baseActivities);

  const expectedBindings = plan.routes.map(temporalMcpDispatchRouteBinding);
  const workflowExecutions = new TemporalMcpWorkflowExecutionCatalog(expectedBindings);
  const dependencies = resolveDependencies();
  const runtime = createRuntime(plan, dependencies);
  assertExactBindings(expectedBindings, runtime.routeBindings);

  return {
    activities: Object.freeze({ ...baseActivities, ...runtime.activities }),
    routeBindings: Object.freeze([...runtime.routeBindings]),
    workflowExecutions,
    runtimeBindingSha256: plan.runtimeBindingSha256,
    subscriptionRoutes: Object.freeze((plan.subscriptionRoutes ?? []).map(route => Object.freeze({ ...route })))
  };
}

export function validateExternalMcpTemporalWorkerCompositionPlan(
  plan: ExternalMcpTemporalWorkerCompositionPlan,
  baseActivities: AgentTaskWorkerActivities
): void {
  if (!sha256Pattern.test(plan.runtimeBindingSha256)) {
    throw new Error("External MCP Temporal Worker Runtime binding is invalid");
  }
  if (externalMcpReadinessBindingSha256(
    plan.workerExternalMcp.config,
    plan.workerExternalMcp.io,
    plan.workerExternalMcp.readinessBindingOptions
  ) !== plan.runtimeBindingSha256) {
    throw new Error("External MCP Temporal Worker Runtime binding is conflicting");
  }
  if ("executeMcpDispatch" in baseActivities) {
    throw new Error("External MCP Temporal Worker Activity collision");
  }

  const expectedBindings = plan.routes.map(temporalMcpDispatchRouteBinding);
  for (const route of plan.routes) plan.routeRegistry.workerEgressPolicies(route.capabilityId);
  new TemporalMcpWorkflowExecutionCatalog(expectedBindings);
  if ((plan.subscriptionRoutes?.length ?? 0) > 0) {
    new TemporalMcpSubscriptionRouteSelector(plan.subscriptionRoutes!);
    const routeIds = new Set(expectedBindings.map(binding => binding.routeId));
    if (plan.subscriptionRoutes!.some(route => !routeIds.has(route.routeId))) {
      throw new Error("External MCP subscription routes conflict with the Worker catalog");
    }
  }
}

function assertExactBindings(
  expected: readonly TemporalMcpDispatchRouteBinding[],
  actual: readonly TemporalMcpDispatchRouteBinding[]
): void {
  if (actual.length !== expected.length || actual.some((binding, index) => {
    const candidate = expected[index];
    return candidate === undefined || binding.routeId !== candidate.routeId ||
      binding.routeVersion !== candidate.routeVersion ||
      binding.routeManifestSha256 !== candidate.routeManifestSha256;
  })) {
    throw new Error("External MCP Temporal Worker route bindings are conflicting");
  }
}
