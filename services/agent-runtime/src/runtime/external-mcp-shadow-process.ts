import type { ExternalMcpDeploymentPlanOptions } from "../mcp/external-mcp-deployment-composition.js";
import type { AgentTaskWorkerActivities } from "../temporal/agent-task-activities.js";
import type { ExternalMcpTemporalRouteSelectorFactory } from "../temporal/external-mcp-temporal-client-lifecycle.js";
import {
  startExternalMcpShadowTemporalRuntime,
  type ExternalMcpShadowTemporalRuntime
} from "../temporal/external-mcp-shadow-temporal-runtime.js";
import type { TemporalRuntimeConfig } from "../temporal/temporal-runtime.js";
import {
  createKafkaShadowRuntime,
  type ShadowRuntime,
  type ShadowRuntimeConfig
} from "./shadow-runtime.js";

export interface ExternalMcpShadowProcess {
  readonly temporal: ExternalMcpShadowTemporalRuntime;
  stop(): Promise<void>;
}

export interface ExternalMcpShadowProcessSeams {
  readonly startTemporal: typeof startExternalMcpShadowTemporalRuntime;
  readonly createKafka: typeof createKafkaShadowRuntime;
}

const defaultSeams: ExternalMcpShadowProcessSeams = {
  startTemporal: startExternalMcpShadowTemporalRuntime,
  createKafka: createKafkaShadowRuntime
};

export async function startExternalMcpShadowProcess(
  env: NodeJS.ProcessEnv,
  shadowConfig: ShadowRuntimeConfig,
  temporalConfig: TemporalRuntimeConfig,
  baseActivities: AgentTaskWorkerActivities,
  createRoutes: ExternalMcpTemporalRouteSelectorFactory,
  options: ExternalMcpDeploymentPlanOptions = {},
  onFailure: (error: unknown) => void = () => undefined,
  seams: ExternalMcpShadowProcessSeams = defaultSeams
): Promise<ExternalMcpShadowProcess | undefined> {
  if (!shadowConfig.enabled) return undefined;
  if (shadowConfig.triggerMode !== "subscription") {
    throw new Error("External MCP Shadow process requires subscription trigger mode");
  }

  const signal = options.signal ?? new AbortController().signal;
  signal.throwIfAborted();
  let temporal: ExternalMcpShadowTemporalRuntime | undefined;
  let kafka: ShadowRuntime | undefined;
  try {
    temporal = await seams.startTemporal(
      env,
      shadowConfig,
      temporalConfig,
      baseActivities,
      createRoutes,
      { ...options, signal },
      onFailure
    );
    if (temporal === undefined) return undefined;
    signal.throwIfAborted();
    if (temporal.subscriptionMatcher === undefined) {
      throw new Error("External MCP subscription matcher is unavailable");
    }
    kafka = seams.createKafka(shadowConfig, temporal, temporal.subscriptionMatcher);
    signal.throwIfAborted();
    await kafka.start();
    signal.throwIfAborted();
  } catch {
    const cleanupFailed = await stopOwned(kafka, temporal);
    if (cleanupFailed) throw new Error("External MCP Shadow process cleanup failed");
    if (signal.aborted) signal.throwIfAborted();
    throw new Error("External MCP Shadow process startup failed");
  }

  return {
    temporal,
    stop: stopOnce(kafka, temporal)
  };
}

function stopOnce(kafka: ShadowRuntime, temporal: ExternalMcpShadowTemporalRuntime): () => Promise<void> {
  let stopPromise: Promise<void> | undefined;
  return () => {
    stopPromise ??= stopOwned(kafka, temporal).then((failed) => {
      if (failed) throw new Error("External MCP Shadow process shutdown failed");
    });
    return stopPromise;
  };
}

async function stopOwned(
  kafka: ShadowRuntime | undefined,
  temporal: ExternalMcpShadowTemporalRuntime | undefined
): Promise<boolean> {
  let failed = false;
  if (kafka !== undefined) {
    try {
      await kafka.stop();
    } catch {
      failed = true;
    }
  }
  if (temporal !== undefined) {
    try {
      await temporal.stop();
    } catch {
      failed = true;
    }
  }
  return failed;
}
