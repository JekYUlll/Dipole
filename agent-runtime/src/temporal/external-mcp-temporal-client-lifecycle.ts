import type { AgentEvent, AgentIdentity, ShadowTaskDispatcher } from "../events/shadow-processor.js";
import type { ExternalMcpTemporalWorkerComposition } from "./external-mcp-temporal-worker-composition.js";
import type { ExternalMcpTemporalWorkerLifecycle } from "./external-mcp-temporal-worker-lifecycle.js";
import {
  TemporalMcpShadowTaskDispatcher,
  type TemporalMcpShadowRouteSelector
} from "./mcp-shadow-task-dispatcher.js";
import {
  TemporalMcpTaskClient,
  type TemporalWorkflowStartPort
} from "./temporal-task-client.js";
import type { TemporalRuntimeConfig } from "./temporal-runtime.js";

export interface ExternalMcpTemporalClientResource {
  readonly workflow: TemporalWorkflowStartPort;
  close(): Promise<void>;
}

export type ExternalMcpTemporalClientResourceFactory = (
  config: TemporalRuntimeConfig,
  signal: AbortSignal
) => Promise<ExternalMcpTemporalClientResource>;

export type ExternalMcpTemporalRouteSelectorFactory = (
  worker: ExternalMcpTemporalWorkerComposition
) => TemporalMcpShadowRouteSelector;

export interface ExternalMcpTemporalClientLifecycle extends ShadowTaskDispatcher {
  stop(): Promise<void>;
}

export interface ExternalMcpTemporalClientLifecycleOptions {
  readonly signal?: AbortSignal;
  readonly createResource?: ExternalMcpTemporalClientResourceFactory;
}

export async function startExternalMcpTemporalClientLifecycle(
  workerLifecycle: ExternalMcpTemporalWorkerLifecycle | undefined,
  createRoutes: ExternalMcpTemporalRouteSelectorFactory,
  options: ExternalMcpTemporalClientLifecycleOptions = {}
): Promise<ExternalMcpTemporalClientLifecycle | undefined> {
  if (workerLifecycle === undefined) return undefined;

  const signal = options.signal ?? new AbortController().signal;
  signal.throwIfAborted();
  let resource: ExternalMcpTemporalClientResource | undefined;
  try {
    const config = workerLifecycle.temporal;
    if (!config.enabled) throw new Error("Temporal is disabled");
    const routes = createRoutes(workerLifecycle.worker);
    signal.throwIfAborted();
    resource = await (options.createResource ?? createNativeResource)(config, signal);
    signal.throwIfAborted();
    const tasks = new TemporalMcpTaskClient(
      resource.workflow,
      config.taskQueue,
      workerLifecycle.worker.workflowExecutions
    );
    return createLifecycle(new TemporalMcpShadowTaskDispatcher(tasks, routes), resource);
  } catch {
    if (resource !== undefined && await closeResource(resource)) {
      throw new Error("External MCP Temporal Client lifecycle cleanup failed");
    }
    if (signal.aborted) signal.throwIfAborted();
    throw new Error("External MCP Temporal Client lifecycle startup failed");
  }
}

function createLifecycle(
  dispatcher: TemporalMcpShadowTaskDispatcher,
  resource: ExternalMcpTemporalClientResource
): ExternalMcpTemporalClientLifecycle {
  let accepting = true;
  let stopPromise: Promise<void> | undefined;
  const inFlight = new Set<Promise<void>>();
  return {
    async dispatch(event: AgentEvent, identity: AgentIdentity, taskId: string): Promise<void> {
      if (!accepting) throw new Error("External MCP Temporal Client lifecycle is not running");
      const operation = dispatcher.dispatch(event, identity, taskId);
      inFlight.add(operation);
      try {
        await operation;
      } finally {
        inFlight.delete(operation);
      }
    },
    stop(): Promise<void> {
      accepting = false;
      stopPromise ??= Promise.allSettled([...inFlight])
        .then(() => resource.close())
        .catch(() => {
          throw new Error("External MCP Temporal Client lifecycle shutdown failed");
        });
      return stopPromise;
    }
  };
}

const createNativeResource: ExternalMcpTemporalClientResourceFactory = async (config, signal) => {
  signal.throwIfAborted();
  const { Client, Connection } = await import("@temporalio/client");
  const connection = await Connection.connect({ address: config.address });
  try {
    signal.throwIfAborted();
    const client = new Client({ connection, namespace: config.namespace });
    return { workflow: client.workflow, close: () => connection.close() };
  } catch (error) {
    await connection.close();
    throw error;
  }
};

async function closeResource(resource: ExternalMcpTemporalClientResource): Promise<boolean> {
  try {
    await resource.close();
    return false;
  } catch {
    return true;
  }
}
