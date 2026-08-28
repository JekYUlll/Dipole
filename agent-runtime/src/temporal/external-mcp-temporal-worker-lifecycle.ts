import type { ExternalMcpDeploymentPlan } from "../mcp/external-mcp-deployment-composition.js";
import type { ShadowSubscriptionMatcher } from "../runtime/shadow-runtime.js";
import type { ExternalMcpTemporalWorkerComposition } from "./external-mcp-temporal-worker-composition.js";
import type { ExternalMcpTemporalWorkerStartupPlan } from "./external-mcp-temporal-worker-startup-plan.js";
import {
  createTemporalWorkerRuntime,
  type TemporalRuntimeConfig,
  type TemporalWorkerActivities,
  type TemporalWorkerRuntime
} from "./temporal-runtime.js";

export type ExternalMcpTemporalWorkerRuntimeFactory = (
  config: TemporalRuntimeConfig,
  activities: TemporalWorkerActivities,
  onFailure: (error: unknown) => void
) => TemporalWorkerRuntime;

export interface ExternalMcpTemporalWorkerLifecycle {
  readonly deployment: ExternalMcpDeploymentPlan;
  readonly worker: ExternalMcpTemporalWorkerComposition;
  readonly temporal: Readonly<TemporalRuntimeConfig>;
  readonly subscriptionMatcher?: ShadowSubscriptionMatcher;
  stop(): Promise<void>;
}

const defaultRuntimeFactory: ExternalMcpTemporalWorkerRuntimeFactory = (config, activities, onFailure) =>
  createTemporalWorkerRuntime(config, activities, undefined, onFailure);

export async function startExternalMcpTemporalWorkerLifecycle(
  startup: ExternalMcpTemporalWorkerStartupPlan | undefined,
  config: TemporalRuntimeConfig,
  onFailure: (error: unknown) => void = () => undefined,
  createRuntime: ExternalMcpTemporalWorkerRuntimeFactory = defaultRuntimeFactory
): Promise<ExternalMcpTemporalWorkerLifecycle | undefined> {
  if (startup === undefined) return undefined;

  let runtime: TemporalWorkerRuntime | undefined;
  try {
    if (!config.enabled) throw new Error("Temporal Worker is disabled");
    runtime = createRuntime(config, startup.worker.activities, onFailure);
    await runtime.start();
  } catch {
    const cleanupFailed = runtime === undefined
      ? await closeStartup(startup)
      : await stopOwned(runtime, startup);
    if (cleanupFailed) {
      throw new Error("External MCP Temporal Worker lifecycle cleanup failed");
    }
    throw new Error("External MCP Temporal Worker lifecycle startup failed");
  }

  return {
    deployment: startup.deployment,
    worker: startup.worker,
    temporal: Object.freeze({ ...config }),
    ...(startup.subscriptionMatcher === undefined ? {} : { subscriptionMatcher: startup.subscriptionMatcher }),
    stop: stopOnce(runtime, startup)
  };
}

function stopOnce(
  runtime: TemporalWorkerRuntime,
  startup: ExternalMcpTemporalWorkerStartupPlan
): () => Promise<void> {
  let stopPromise: Promise<void> | undefined;
  return () => {
    stopPromise ??= stopOwned(runtime, startup).then((failed) => {
      if (failed) throw new Error("External MCP Temporal Worker lifecycle shutdown failed");
    });
    return stopPromise;
  };
}

async function stopOwned(
  runtime: TemporalWorkerRuntime,
  startup: ExternalMcpTemporalWorkerStartupPlan
): Promise<boolean> {
  let failed = false;
  try {
    await runtime.stop();
  } catch {
    failed = true;
  }
  try {
    await startup.close();
  } catch {
    failed = true;
  }
  return failed;
}

async function closeStartup(startup: ExternalMcpTemporalWorkerStartupPlan): Promise<boolean> {
  try {
    await startup.close();
    return false;
  } catch {
    return true;
  }
}
