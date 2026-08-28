import type { ExternalMcpCapabilityDefinitionRegistry } from "../mcp/external-mcp-deployment-route-manifest.js";
import {
  loadExternalMcpDeploymentPlan,
  type ExternalMcpDeploymentPlan,
  type ExternalMcpDeploymentPlanOptions
} from "../mcp/external-mcp-deployment-composition.js";
import type { AgentTaskWorkerActivities } from "./agent-task-activities.js";
import {
  createExternalMcpTemporalWorkerComposition,
  type ExternalMcpTemporalWorkerComposition,
  validateExternalMcpTemporalWorkerCompositionPlan
} from "./external-mcp-temporal-worker-composition.js";
import type { TemporalMcpMultiRouteRuntimeDependencies } from "./mcp-multi-route-runtime.js";

export interface ExternalMcpTemporalWorkerResource {
  readonly dependencies: TemporalMcpMultiRouteRuntimeDependencies;
  readonly workerActivities?: AgentTaskWorkerActivities;
  close(): Promise<void> | void;
}

export type ExternalMcpTemporalWorkerResourceFactory = (
  plan: ExternalMcpDeploymentPlan,
  signal: AbortSignal
) => Promise<ExternalMcpTemporalWorkerResource>;

export interface ExternalMcpTemporalWorkerStartupPlan {
  readonly deployment: ExternalMcpDeploymentPlan;
  readonly worker: ExternalMcpTemporalWorkerComposition;
  close(): Promise<void>;
}

export interface ExternalMcpTemporalWorkerStartupSeams {
  readonly loadDeployment: typeof loadExternalMcpDeploymentPlan;
  readonly validateCompositionPlan: typeof validateExternalMcpTemporalWorkerCompositionPlan;
  readonly compose: typeof createExternalMcpTemporalWorkerComposition;
}

const defaultSeams: ExternalMcpTemporalWorkerStartupSeams = {
  loadDeployment: loadExternalMcpDeploymentPlan,
  validateCompositionPlan: validateExternalMcpTemporalWorkerCompositionPlan,
  compose: createExternalMcpTemporalWorkerComposition
};

export async function loadExternalMcpTemporalWorkerStartupPlan(
  definitions: ExternalMcpCapabilityDefinitionRegistry,
  env: NodeJS.ProcessEnv,
  baseActivities: AgentTaskWorkerActivities,
  createResource: ExternalMcpTemporalWorkerResourceFactory,
  options: ExternalMcpDeploymentPlanOptions = {},
  seams: ExternalMcpTemporalWorkerStartupSeams = defaultSeams
): Promise<ExternalMcpTemporalWorkerStartupPlan | undefined> {
  const signal = options.signal ?? new AbortController().signal;
  signal.throwIfAborted();
  let resource: ExternalMcpTemporalWorkerResource | undefined;
  try {
    const deployment = await seams.loadDeployment(definitions, env, { ...options, signal });
    signal.throwIfAborted();
    if (deployment === undefined) return undefined;

    seams.validateCompositionPlan(deployment, baseActivities);
    signal.throwIfAborted();
    resource = await createResource(deployment, signal);
    signal.throwIfAborted();
    const worker = seams.compose(
      deployment,
      resource.workerActivities ?? baseActivities,
      () => resource!.dependencies
    );
    signal.throwIfAborted();
    if (worker === undefined) throw new Error("missing Worker composition");

    return {
      deployment,
      worker,
      close: closeOnce(resource)
    };
  } catch {
    let cleanupFailed = false;
    if (resource !== undefined) {
      try {
        await resource.close();
      } catch {
        cleanupFailed = true;
      }
    }
    if (cleanupFailed) {
      throw new Error("External MCP Temporal Worker resource cleanup failed");
    }
    if (signal.aborted) signal.throwIfAborted();
    throw new Error("External MCP Temporal Worker startup plan is unavailable");
  }
}

function closeOnce(resource: ExternalMcpTemporalWorkerResource): () => Promise<void> {
  let closePromise: Promise<void> | undefined;
  return () => {
    closePromise ??= Promise.resolve()
      .then(() => resource.close())
      .catch(() => {
        throw new Error("External MCP Temporal Worker resource cleanup failed");
      });
    return closePromise;
  };
}
