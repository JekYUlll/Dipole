import type { AgentCapabilityRPCClient } from "../capabilities/agent-capability-rpc.js";
import {
  createAgentCapabilityRPC,
  type ShadowRuntimeConfig
} from "../runtime/shadow-runtime.js";
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

export function createExternalMcpAgentCapabilityRPCResourceFactory(
  config: ShadowRuntimeConfig,
  createRPC: ExternalMcpAgentCapabilityRPCFactory = createAgentCapabilityRPC
): ExternalMcpTemporalWorkerResourceFactory {
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
