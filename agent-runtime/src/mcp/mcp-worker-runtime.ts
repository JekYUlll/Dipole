import type { AgentMcpToolCommand } from "../capabilities/agent-capability-rpc.js";
import {
  ExternalMcpActivityRoundSessionFactory,
  McpInputRequiredActivity,
  type McpActivityExternalTransportRegistry,
  type McpActivityModernClientFactory,
  type McpToolRoundReceiptClient
} from "./mcp-input-required-activity.js";
import type { McpToolEgressPolicy } from "./mcp-tool-client.js";
import { McpWorkerCommandDispatcher } from "./mcp-worker-dispatch.js";
import {
  McpTerminalWorkerRuntime,
  type McpInvocationTerminalClient
} from "./mcp-terminal-worker-runtime.js";

export interface McpWorkerCoreClient extends McpToolRoundReceiptClient {
  resolveMcpToolCommand(taskId: string, runId: string, invocationId: string): Promise<AgentMcpToolCommand>;
}

export interface McpWorkerRuntimeDependencies {
  readonly core: McpWorkerCoreClient;
  readonly transports: McpActivityExternalTransportRegistry;
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
  const sessions = new ExternalMcpActivityRoundSessionFactory(
    dependencies.transports,
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
