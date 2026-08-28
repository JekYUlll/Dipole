import type { AgentCapabilityRPCClient } from "../capabilities/agent-capability-rpc.js";
import {
  createAgentCapabilityRPC,
  type ShadowRuntimeConfig
} from "../runtime/shadow-runtime.js";
import {
  foundationAgentTaskActivities,
  type AgentTaskWorkerActivities
} from "./agent-task-activities.js";
import { createPersistentAgentTaskLifecycleActivities } from "./agent-task-lifecycle-activities.js";
import type {
  ExternalMcpTemporalWorkerResource,
  ExternalMcpTemporalWorkerResourceFactory
} from "./external-mcp-temporal-worker-startup-plan.js";

export interface ExternalMcpAgentCapabilityRPCResource {
  readonly client: AgentCapabilityRPCClient;
  close(): Promise<void> | void;
}

export type ExternalMcpAgentCapabilityRPCFactory = (
  config: ShadowRuntimeConfig
) => ExternalMcpAgentCapabilityRPCResource;

export interface ExternalMcpAgentCapabilityRPCResourceFactoryOptions {
  readonly createRPC?: ExternalMcpAgentCapabilityRPCFactory;
  readonly baseActivities?: AgentTaskWorkerActivities;
}

export function createExternalMcpAgentCapabilityRPCResourceFactory(
  config: ShadowRuntimeConfig,
  options: ExternalMcpAgentCapabilityRPCResourceFactoryOptions = {}
): ExternalMcpTemporalWorkerResourceFactory {
  const createRPC = options.createRPC ?? createAgentCapabilityRPC;
  const baseActivities = options.baseActivities ?? foundationAgentTaskActivities;
  return async (plan, signal) => {
    signal.throwIfAborted();
    let rpc: ExternalMcpAgentCapabilityRPCResource | undefined;
    try {
      if (!config.capabilityRpc.enabled || plan.config.profiles.length === 0 ||
          plan.config.profiles.some(profile => profile.tenantId !== config.tenantId)) {
        throw new Error("invalid RPC resource authority");
      }
      signal.throwIfAborted();
      rpc = createRPC(config);
      signal.throwIfAborted();
      return {
        dependencies: Object.freeze({ core: rpc.client, artifacts: rpc.client }),
        subscriptionMatcher: rpc.client,
        workerActivities: Object.freeze({
          ...baseActivities,
          ...createPersistentAgentTaskLifecycleActivities(rpc.client)
        }),
        close: closeOnce(rpc)
      } satisfies ExternalMcpTemporalWorkerResource;
    } catch {
      const cleanupFailed = rpc === undefined ? false : await closeResource(rpc);
      if (cleanupFailed) {
        throw new Error("External MCP Agent Capability RPC resource cleanup failed");
      }
      if (signal.aborted) signal.throwIfAborted();
      throw new Error("External MCP Agent Capability RPC resource is unavailable");
    }
  };
}

function closeOnce(resource: ExternalMcpAgentCapabilityRPCResource): () => Promise<void> {
  let closePromise: Promise<void> | undefined;
  return () => {
    closePromise ??= Promise.resolve()
      .then(() => resource.close())
      .catch(() => {
        throw new Error("External MCP Agent Capability RPC resource cleanup failed");
      });
    return closePromise;
  };
}

async function closeResource(resource: ExternalMcpAgentCapabilityRPCResource): Promise<boolean> {
  try {
    await resource.close();
    return false;
  } catch {
    return true;
  }
}
