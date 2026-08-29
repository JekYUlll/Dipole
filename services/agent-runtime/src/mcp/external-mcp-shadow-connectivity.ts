import type { Tool, Transport } from "@modelcontextprotocol/client";

import { AllowlistedMcpToolClient, type McpToolEgressPolicy } from "./mcp-tool-client.js";

export const externalMcpShadowConnectivityReceiptSchemaVersion =
  "dipole.agent.external-mcp-shadow-connectivity.v1" as const;

interface ExternalMcpShadowConnectivityProfile {
  readonly profileId: string;
  readonly tenantId: string;
  readonly serverId: string;
  readonly allowedTools: readonly string[];
}

export interface ExternalMcpShadowConnectivityRegistry {
  describe(profileId: string, tenantId: string): ExternalMcpShadowConnectivityProfile;
  connect(profileId: string, tenantId: string, signal?: AbortSignal): Promise<Transport>;
}

export interface ExternalMcpShadowConnectivityClient {
  connect(transport: Transport, signal?: AbortSignal): Promise<readonly Tool[]>;
  close(): Promise<void>;
}

export interface ExternalMcpShadowConnectivityClientConfig {
  readonly serverId: string;
  readonly allowedTools: readonly string[];
  readonly requestTimeoutMs: number;
}

export interface ExternalMcpShadowConnectivityDrillOptions {
  readonly requestTimeoutMs?: number;
  readonly now?: () => Date;
  readonly createClient?: (
    config: ExternalMcpShadowConnectivityClientConfig
  ) => ExternalMcpShadowConnectivityClient;
}

export interface ExternalMcpShadowConnectivityReceipt {
  readonly schemaVersion: typeof externalMcpShadowConnectivityReceiptSchemaVersion;
  readonly checkedAt: string;
  readonly toolCount: number;
}

export type ExternalMcpShadowConnectivityDrill = (
  input: { readonly profileId: string; readonly tenantId: string },
  signal?: AbortSignal
) => Promise<ExternalMcpShadowConnectivityReceipt>;

export function createExternalMcpShadowConnectivityDrill(
  registry: ExternalMcpShadowConnectivityRegistry,
  options: ExternalMcpShadowConnectivityDrillOptions = {}
): ExternalMcpShadowConnectivityDrill {
  const requestTimeoutMs = options.requestTimeoutMs ?? 10_000;
  if (!Number.isSafeInteger(requestTimeoutMs) || requestTimeoutMs < 100 || requestTimeoutMs > 60_000) {
    throw new Error("External MCP Shadow connectivity timeout must be between 100 and 60000 milliseconds");
  }
  const now = options.now ?? (() => new Date());
  const createClient = options.createClient ?? defaultClientFactory;

  return async (input, signal = new AbortController().signal) => {
    signal.throwIfAborted();
    let client: ExternalMcpShadowConnectivityClient | undefined;
    let transport: Transport | undefined;
    let connected = false;
    let result: ExternalMcpShadowConnectivityReceipt | undefined;
    let failed = false;
    try {
      const checkedAt = now();
      if (!Number.isFinite(checkedAt.getTime())) throw new Error("invalid drill clock");
      const profile = registry.describe(input.profileId, input.tenantId);
      if (profile.profileId !== input.profileId || profile.tenantId !== input.tenantId) {
        throw new Error("Shadow connectivity Profile binding mismatch");
      }
      client = createClient({
        serverId: profile.serverId,
        allowedTools: [...profile.allowedTools],
        requestTimeoutMs
      });
      transport = await registry.connect(profile.profileId, profile.tenantId, signal);
      signal.throwIfAborted();
      const tools = await client.connect(transport, signal);
      connected = true;
      signal.throwIfAborted();
      assertCompleteDiscovery(profile.allowedTools, tools);
      result = {
        schemaVersion: externalMcpShadowConnectivityReceiptSchemaVersion,
        checkedAt: checkedAt.toISOString(),
        toolCount: tools.length
      };
    } catch {
      failed = true;
    }

    const cleanup = await Promise.allSettled([
      ...(client === undefined ? [] : [Promise.resolve().then(() => client!.close())]),
      ...(transport === undefined || connected ? [] : [Promise.resolve().then(() => transport!.close())])
    ]);
    if (signal.aborted) signal.throwIfAborted();
    if (failed || result === undefined || cleanup.some(item => item.status === "rejected")) {
      throw new Error("External MCP Shadow connectivity drill failed");
    }
    return result;
  };
}

function defaultClientFactory(config: ExternalMcpShadowConnectivityClientConfig): ExternalMcpShadowConnectivityClient {
  const egressPolicies = Object.fromEntries(config.allowedTools.map(name => [name, {
    allowedArgumentNames: [],
    maximumBytes: 2
  } satisfies McpToolEgressPolicy]));
  return new AllowlistedMcpToolClient(
    config.serverId,
    [config.serverId],
    config.allowedTools,
    egressPolicies,
    config.requestTimeoutMs,
    "modern"
  );
}

function assertCompleteDiscovery(allowedTools: readonly string[], tools: readonly Tool[]): void {
  const discovered = new Set(tools.map(tool => tool.name));
  if (tools.length !== allowedTools.length || discovered.size !== tools.length ||
      allowedTools.some(name => !discovered.has(name))) {
    throw new Error("External MCP Shadow connectivity discovery is incomplete");
  }
}
