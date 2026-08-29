import type {
  ExternalMcpDeploymentPlan,
  ExternalMcpDeploymentPlanOptions
} from "../mcp/external-mcp-deployment-composition.js";
import type { AgentEvent, AgentIdentity, ShadowTaskDispatcher } from "../events/shadow-processor.js";
import type { ShadowRuntimeConfig, ShadowSubscriptionMatcher } from "../runtime/shadow-runtime.js";
import type { AgentTaskWorkerActivities } from "./agent-task-activities.js";
import {
  startExternalMcpTemporalClientLifecycle,
  type ExternalMcpTemporalClientLifecycle,
  type ExternalMcpTemporalRouteSelectorFactory
} from "./external-mcp-temporal-client-lifecycle.js";
import {
  startExternalMcpShadowWorkerBootstrap
} from "./external-mcp-shadow-worker-bootstrap.js";
import type { ExternalMcpTemporalWorkerComposition } from "./external-mcp-temporal-worker-composition.js";
import type { TemporalRuntimeConfig } from "./temporal-runtime.js";

export interface ExternalMcpShadowTemporalRuntime extends ShadowTaskDispatcher {
  readonly deployment: ExternalMcpDeploymentPlan;
  readonly worker: ExternalMcpTemporalWorkerComposition;
  readonly temporal: Readonly<TemporalRuntimeConfig>;
  readonly subscriptionMatcher?: ShadowSubscriptionMatcher;
  stop(): Promise<void>;
}

export interface ExternalMcpShadowTemporalRuntimeSeams {
  readonly startWorker: typeof startExternalMcpShadowWorkerBootstrap;
  readonly startClient: typeof startExternalMcpTemporalClientLifecycle;
}

const defaultSeams: ExternalMcpShadowTemporalRuntimeSeams = {
  startWorker: startExternalMcpShadowWorkerBootstrap,
  startClient: startExternalMcpTemporalClientLifecycle
};

export async function startExternalMcpShadowTemporalRuntime(
  env: NodeJS.ProcessEnv,
  shadowConfig: ShadowRuntimeConfig,
  temporalConfig: TemporalRuntimeConfig,
  baseActivities: AgentTaskWorkerActivities,
  createRoutes: ExternalMcpTemporalRouteSelectorFactory,
  options: ExternalMcpDeploymentPlanOptions = {},
  onFailure: (error: unknown) => void = () => undefined,
  seams: ExternalMcpShadowTemporalRuntimeSeams = defaultSeams
): Promise<ExternalMcpShadowTemporalRuntime | undefined> {
  const signal = options.signal ?? new AbortController().signal;
  signal.throwIfAborted();
  let worker: Awaited<ReturnType<typeof startExternalMcpShadowWorkerBootstrap>> = undefined;
  let client: ExternalMcpTemporalClientLifecycle | undefined;
  try {
    worker = await seams.startWorker(
      env,
      shadowConfig,
      temporalConfig,
      baseActivities,
      { ...options, signal },
      onFailure
    );
    if (worker === undefined) return undefined;
    signal.throwIfAborted();
    client = await seams.startClient(worker, createRoutes, { signal });
    if (client === undefined) throw new Error("External MCP Temporal Client is unavailable");
    signal.throwIfAborted();
  } catch {
    const cleanupFailed = await stopOwned(client, worker);
    if (cleanupFailed) throw new Error("External MCP Shadow Temporal runtime cleanup failed");
    if (signal.aborted) signal.throwIfAborted();
    throw new Error("External MCP Shadow Temporal runtime startup failed");
  }

  return {
    deployment: worker.deployment,
    worker: worker.worker,
    temporal: worker.temporal,
    ...(worker.subscriptionMatcher === undefined ? {} : { subscriptionMatcher: worker.subscriptionMatcher }),
    dispatch: (event: AgentEvent, identity: AgentIdentity, taskId: string) => client.dispatch(event, identity, taskId),
    stop: stopOnce(client, worker)
  };
}

function stopOnce(
  client: ExternalMcpTemporalClientLifecycle,
  worker: NonNullable<Awaited<ReturnType<typeof startExternalMcpShadowWorkerBootstrap>>>
): () => Promise<void> {
  let stopPromise: Promise<void> | undefined;
  return () => {
    stopPromise ??= stopOwned(client, worker).then((failed) => {
      if (failed) throw new Error("External MCP Shadow Temporal runtime shutdown failed");
    });
    return stopPromise;
  };
}

async function stopOwned(
  client: ExternalMcpTemporalClientLifecycle | undefined,
  worker: Awaited<ReturnType<typeof startExternalMcpShadowWorkerBootstrap>>
): Promise<boolean> {
  let failed = false;
  if (client !== undefined) {
    try {
      await client.stop();
    } catch {
      failed = true;
    }
  }
  if (worker !== undefined) {
    try {
      await worker.stop();
    } catch {
      failed = true;
    }
  }
  return failed;
}
