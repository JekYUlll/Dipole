import type { AgentMcpToolCommand } from "../capabilities/agent-capability-rpc.js";
import {
  ExternalMcpActivityRoundSessionFactory,
  McpInputRequiredActivity,
  type McpActivityModernClientFactory,
  type McpToolRoundReceiptClient
} from "./mcp-input-required-activity.js";
import type { McpToolEgressPolicy } from "./mcp-tool-client.js";
import type { ExternalMcpConfig } from "./external-mcp-profile.js";
import type { ExternalMcpProductionIoConfig } from "./external-mcp-production-io.js";
import {
  ExternalMcpReadinessGatedTransportRegistry,
  type ExternalMcpFreshReadinessResolver,
  type ExternalMcpReadinessUnderlyingRegistry
} from "./external-mcp-readiness-egress.js";
import type { ExternalMcpReadinessBindingOptions } from "./external-mcp-readiness-evidence.js";
import { McpWorkerCommandDispatcher } from "./mcp-worker-dispatch.js";
import {
  McpTerminalWorkerRuntime,
  type McpInvocationTerminalClient
} from "./mcp-terminal-worker-runtime.js";

export interface McpWorkerCoreClient extends McpToolRoundReceiptClient, ExternalMcpFreshReadinessResolver {
  resolveMcpToolCommand(taskId: string, runId: string, invocationId: string): Promise<AgentMcpToolCommand>;
}

export interface McpWorkerExternalMcpDependencies {
  readonly config: ExternalMcpConfig;
  readonly io: ExternalMcpProductionIoConfig;
  readonly registry: ExternalMcpReadinessUnderlyingRegistry;
  readonly readinessBindingOptions?: ExternalMcpReadinessBindingOptions;
}

export interface McpWorkerRuntimeDependencies {
  readonly core: McpWorkerCoreClient;
  readonly externalMcp: McpWorkerExternalMcpDependencies;
  readonly egressPolicies: Readonly<Record<string, Readonly<Record<string, McpToolEgressPolicy>>>>;
  readonly requestTimeoutMs?: number;
  readonly inputWindowMs?: number;
  readonly now?: () => number;
  readonly ownerTokenSha256?: () => string;
  readonly createClient?: McpActivityModernClientFactory;
}

export interface McpTerminalWorkerRuntimeDependencies extends Omit<McpWorkerRuntimeDependencies, "core"> {
  readonly core: McpWorkerCoreClient & McpInvocationTerminalClient;
}

export function createMcpWorkerRuntime(dependencies: McpWorkerRuntimeDependencies): McpWorkerCommandDispatcher {
  const transports = new ExternalMcpReadinessGatedTransportRegistry(
    dependencies.externalMcp.config,
    dependencies.externalMcp.io,
    dependencies.externalMcp.registry,
    dependencies.core,
    dependencies.externalMcp.readinessBindingOptions,
    dependencies.now
  );
  const sessions = new ExternalMcpActivityRoundSessionFactory(
    transports,
    dependencies.egressPolicies,
    dependencies.requestTimeoutMs,
    dependencies.createClient
  );
  const activity = new McpInputRequiredActivity(
    sessions,
    dependencies.core,
    dependencies.now,
    dependencies.ownerTokenSha256
  );
  return new McpWorkerCommandDispatcher(
    dependencies.core,
    activity,
    dependencies.now,
    dependencies.inputWindowMs
  );
}

export function createMcpTerminalWorkerRuntime(dependencies: McpTerminalWorkerRuntimeDependencies): McpTerminalWorkerRuntime {
  return new McpTerminalWorkerRuntime(createMcpWorkerRuntime(dependencies), dependencies.core);
}
