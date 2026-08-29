import { createExternalMcpReadCapabilityDefinitions } from "../mcp/external-mcp-read-capability-definitions.js";
import type { ExternalMcpDeploymentPlanOptions } from "../mcp/external-mcp-deployment-composition.js";
import type { ShadowRuntimeConfig } from "../runtime/shadow-runtime.js";
import {
  createExternalMcpAgentCapabilityRPCResourceFactory
} from "./external-mcp-agent-capability-rpc-resource.js";
import type { AgentTaskWorkerActivities } from "./agent-task-activities.js";
import {
  startExternalMcpTemporalWorkerLifecycle,
  type ExternalMcpTemporalWorkerLifecycle
} from "./external-mcp-temporal-worker-lifecycle.js";
import {
  loadExternalMcpTemporalWorkerStartupPlan,
  type ExternalMcpTemporalWorkerResourceFactory,
  type ExternalMcpTemporalWorkerStartupPlan
} from "./external-mcp-temporal-worker-startup-plan.js";
import type { TemporalRuntimeConfig } from "./temporal-runtime.js";

export interface ExternalMcpShadowWorkerBootstrapSeams {
  readonly createDefinitions: typeof createExternalMcpReadCapabilityDefinitions;
  readonly createResourceFactory: typeof createExternalMcpAgentCapabilityRPCResourceFactory;
  readonly loadStartup: typeof loadExternalMcpTemporalWorkerStartupPlan;
  readonly startLifecycle: typeof startExternalMcpTemporalWorkerLifecycle;
}

const defaultSeams: ExternalMcpShadowWorkerBootstrapSeams = {
  createDefinitions: createExternalMcpReadCapabilityDefinitions,
  createResourceFactory: createExternalMcpAgentCapabilityRPCResourceFactory,
  loadStartup: loadExternalMcpTemporalWorkerStartupPlan,
  startLifecycle: startExternalMcpTemporalWorkerLifecycle
};

export async function startExternalMcpShadowWorkerBootstrap(
  env: NodeJS.ProcessEnv,
  shadowConfig: ShadowRuntimeConfig,
  temporalConfig: TemporalRuntimeConfig,
  baseActivities: AgentTaskWorkerActivities,
  options: ExternalMcpDeploymentPlanOptions = {},
  onFailure: (error: unknown) => void = () => undefined,
  seams: ExternalMcpShadowWorkerBootstrapSeams = defaultSeams
): Promise<ExternalMcpTemporalWorkerLifecycle | undefined> {
  const signal = options.signal ?? new AbortController().signal;
  signal.throwIfAborted();
  let startup: ExternalMcpTemporalWorkerStartupPlan | undefined;
  try {
    const definitions = seams.createDefinitions();
    let createResource: ExternalMcpTemporalWorkerResourceFactory | undefined;
    startup = await seams.loadStartup(
      definitions,
      env,
      baseActivities,
      (plan, resourceSignal) => {
        createResource ??= seams.createResourceFactory(shadowConfig, { baseActivities });
        return createResource(plan, resourceSignal);
      },
      { ...options, signal }
    );
    signal.throwIfAborted();
  } catch {
    const cleanupFailed = startup === undefined ? false : await closeStartup(startup);
    if (cleanupFailed) throw new Error("External MCP Shadow Worker bootstrap cleanup failed");
    if (signal.aborted) signal.throwIfAborted();
    throw new Error("External MCP Shadow Worker bootstrap is unavailable");
  }

  if (startup === undefined) return undefined;
  return seams.startLifecycle(startup, temporalConfig, onFailure);
}

async function closeStartup(startup: ExternalMcpTemporalWorkerStartupPlan): Promise<boolean> {
  try {
    await startup.close();
    return false;
  } catch {
    return true;
  }
}
