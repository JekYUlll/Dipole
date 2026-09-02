import { buildServer } from "./server.js";
import { AgentTaskControlService } from "./control/agent-task-control.js";
import { InteractiveTaskStartService } from "./task/interactive-task-request.js";
import { z } from "zod";
import { ConversationListCapability } from "./capabilities/conversation-list.js";
import { ConversationReadCapability } from "./capabilities/conversation-read.js";
import { CapabilityRegistry } from "./capabilities/registry.js";
import { createDipoleMcpHttpHandler } from "./mcp/dipole-mcp-http.js";
import { McpToolInvocationRunner } from "./mcp/mcp-tool-invocation.js";
import {
  createAgentCapabilityRPC,
  createKafkaShadowRuntime,
  createTemporalReadActivityResources,
  loadShadowRuntimeConfig
} from "./runtime/shadow-runtime.js";
import {
  foundationAgentTaskActivities,
  type AgentTaskWorkerActivities
} from "./temporal/agent-task-activities.js";
import { createPersistentAgentTaskLifecycleActivities } from "./temporal/agent-task-lifecycle-activities.js";
import {
  createTemporalTaskDispatchRuntime,
  type TemporalTaskDispatchRuntime
} from "./temporal/temporal-task-client.js";
import {
  createTemporalWorkerRuntime,
  loadTemporalRuntimeConfig,
  type TemporalWorkerRuntime
} from "./temporal/temporal-runtime.js";
import {
  createAgentObservabilityRuntime,
  loadAgentObservabilityConfig
} from "./observability/agent-observability-runtime.js";
import {
  startExternalMcpProductionShadow,
  validateExternalMcpProductionShadowMode
} from "./runtime/external-mcp-production-shadow.js";
import type { ExternalMcpShadowProcess } from "./runtime/external-mcp-shadow-process.js";
import { SubscriptionShadowMetrics } from "./observability/subscription-shadow-metrics.js";
import { assertActiveReadProfileSurface } from "./runtime/active-read-profile.js";
import { assertActiveInteractiveProfile } from "./runtime/active-interactive-profile.js";
import { assertActiveMemoryPromotionProfile } from "./runtime/active-memory-promotion-profile.js";
import { readFileSync } from "node:fs";
import { assertActivePromotionBinding } from "./promotion/agent-release-manifest.js";
import { createAgentMemoryPromotionCommitActivities } from "./temporal/agent-memory-promotion-commit-activity.js";
import { assertOAuthCallbackRuntimeUnavailable, loadOAuthCallbackRuntimeConfig } from "./mcp/oauth-callback-runtime-config.js";

const port = Number.parseInt(process.env.DIPOLE_AGENT_PORT ?? "8091", 10);
const host = process.env.DIPOLE_AGENT_HOST?.trim() || "0.0.0.0";
let ready = false;
const shadowConfig = loadShadowRuntimeConfig(process.env);
const temporalConfig = loadTemporalRuntimeConfig(process.env);
assertOAuthCallbackRuntimeUnavailable(loadOAuthCallbackRuntimeConfig(process.env));
if (shadowConfig.runtimeMode === "active") {
  let releaseManifest: unknown;
  try {
    releaseManifest = JSON.parse(readFileSync(shadowConfig.releaseManifestPath, "utf8"));
  } catch (error) {
    throw new Error(`Active Agent Runtime release manifest cannot be loaded: ${error instanceof Error ? error.message : String(error)}`);
  }
  assertActivePromotionBinding(releaseManifest, shadowConfig.candidateVersion);
}
const memoryPromotionCommitEnabled = process.env.DIPOLE_AGENT_MEMORY_PROMOTION_COMMIT_ENABLED?.trim().toLowerCase() === "true";
const memoryPromotionAuthority = process.env.DIPOLE_AGENT_MEMORY_PROMOTION_AUTHORITY?.trim() || "";
if (shadowConfig.runtimeMode === "active" && temporalConfig.activityMode !== "read_active" && temporalConfig.activityMode !== "interactive_active" && temporalConfig.activityMode !== "promotion_active") {
	throw new Error("Active Agent Runtime requires read_active, interactive_active, or promotion_active Temporal Activities");
}
const controlEnabled = process.env.DIPOLE_AGENT_CONTROL_ENABLED?.trim().toLowerCase() === "true";
const controlSecret = process.env.DIPOLE_AGENT_CONTROL_SECRET ?? process.env.DIPOLE_INTERNAL_RPC_SHARED_SECRET ?? "";
const mcpEnabled = process.env.DIPOLE_AGENT_MCP_SERVER_ENABLED?.trim().toLowerCase() === "true";
const mcpSecret = process.env.DIPOLE_AGENT_MCP_SERVER_SECRET ?? process.env.DIPOLE_INTERNAL_RPC_SHARED_SECRET ?? "";
const externalMcpEnabled = process.env.DIPOLE_AGENT_EXTERNAL_MCP_ENABLED?.trim().toLowerCase() === "true";
const activeReadSurface = {
  controlEnabled,
  mcpServerEnabled: mcpEnabled,
  externalMcpEnabled,
  memoryEnabled: shadowConfig.memoryEnabled,
  retrievalEnabled: shadowConfig.retrievalEnabled,
  retrievalContextEnabled: shadowConfig.retrievalContextEnabled,
  subscriptionShadowEnabled: shadowConfig.subscriptionShadowEnabled
};
if (memoryPromotionCommitEnabled || temporalConfig.activityMode === "promotion_active") {
  assertActiveMemoryPromotionProfile({
    runtimeMode: shadowConfig.runtimeMode,
    temporal: temporalConfig,
    capabilityRPCEnabled: shadowConfig.capabilityRpc.enabled,
    capabilityRPCTLS: shadowConfig.capabilityRpc.tls.enabled,
    commitEnabled: memoryPromotionCommitEnabled,
    authority: memoryPromotionAuthority,
    ...activeReadSurface
  });
} else if (temporalConfig.activityMode === "interactive_active") {
  assertActiveInteractiveProfile({
    runtimeMode: shadowConfig.runtimeMode,
    temporal: temporalConfig,
    capabilityRPCEnabled: shadowConfig.capabilityRpc.enabled,
    capabilityRPCTLS: shadowConfig.capabilityRpc.tls.enabled,
    interactiveMessageWritesEnabled: shadowConfig.interactiveMessageWritesEnabled,
    ...activeReadSurface
  });
} else {
  assertActiveReadProfileSurface(shadowConfig.runtimeMode, activeReadSurface);
}
if (shadowConfig.runtimeMode === "active" && !temporalConfig.enabled) {
  throw new Error("Active Agent Runtime requires Temporal");
}
const observabilityRuntime = createAgentObservabilityRuntime(loadAgentObservabilityConfig(process.env));
const subscriptionShadowMetrics = new SubscriptionShadowMetrics(shadowConfig.subscriptionShadowEnabled);
const externalMcpEnvironment = Object.freeze({ ...process.env });
const externalMcpShadowEnabled = validateExternalMcpProductionShadowMode(
  externalMcpEnvironment,
  shadowConfig,
  temporalConfig
);
const mcpResource = process.env.DIPOLE_AGENT_MCP_RESOURCE?.trim() || "https://dipole.local/api/v1/agent/mcp";
const mcpToolTimeoutMs = Number.parseInt(process.env.DIPOLE_AGENT_MCP_TOOL_TIMEOUT_MS ?? "5000", 10);
if (controlEnabled && (!temporalConfig.enabled || !shadowConfig.capabilityRpc.enabled || controlSecret.trim().length === 0)) {
  throw new Error("Agent Task controls require Temporal, Agent Capability RPC, and a control secret");
}
if (mcpEnabled && (!shadowConfig.capabilityRpc.enabled || mcpSecret.trim().length === 0)) {
  throw new Error("Agent MCP Server requires Agent Capability RPC and an MCP server secret");
}
if (mcpEnabled) {
  const resource = new URL(mcpResource);
  if ((resource.protocol !== "https:" && resource.protocol !== "http:") || resource.username !== "" || resource.password !== "" || resource.hash !== "" || resource.search !== "") {
    throw new Error("Agent MCP resource must be a canonical HTTP(S) URI without credentials, query, or fragment");
  }
  if (!Number.isSafeInteger(mcpToolTimeoutMs) || mcpToolTimeoutMs < 100 || mcpToolTimeoutMs > 60_000) {
    throw new Error("Agent MCP Tool timeout must be between 100 and 60000 milliseconds");
  }
}
observabilityRuntime.start();
let temporalRuntime: TemporalWorkerRuntime | undefined;
let temporalRPC: ReturnType<typeof createAgentCapabilityRPC> | undefined;
const controlRPC = controlEnabled ? createAgentCapabilityRPC(shadowConfig) : undefined;
const mcpRPC = mcpEnabled ? createAgentCapabilityRPC(shadowConfig) : undefined;
const temporalReadResources = temporalConfig.enabled && (temporalConfig.activityMode === "read_shadow" || temporalConfig.activityMode === "read_active" || temporalConfig.activityMode === "interactive_active" || temporalConfig.activityMode === "promotion_active")
  ? createTemporalReadActivityResources(shadowConfig)
  : undefined;
let temporalDispatcher: TemporalTaskDispatchRuntime | undefined;
if (temporalConfig.enabled && (((temporalConfig.activityMode === "read_shadow" || temporalConfig.activityMode === "read_active" || temporalConfig.activityMode === "interactive_active" || temporalConfig.activityMode === "promotion_active") && shadowConfig.enabled) || controlEnabled)) {
  temporalDispatcher = createTemporalTaskDispatchRuntime(temporalConfig);
}
const shadowRuntime = shadowConfig.enabled && !externalMcpShadowEnabled
  ? createKafkaShadowRuntime(
    shadowConfig, temporalDispatcher, undefined,
    shadowConfig.subscriptionShadowEnabled ? subscriptionShadowMetrics : undefined
  )
  : undefined;
let externalMcpShadowProcess: ExternalMcpShadowProcess | undefined;
let serverStarted = false;
let shadowStarted = false;
let temporalStarted = false;
let temporalReadResourcesOpen = temporalReadResources !== undefined;
let temporalDispatcherStarted = false;
let stopPromise: Promise<void> | undefined;

const interactiveTaskStarter = controlEnabled
  ? new InteractiveTaskStartService({ tenantId: shadowConfig.tenantId, agentId: shadowConfig.agentUuid }, temporalDispatcher!)
  : undefined;
const controlService = controlEnabled
  ? Object.assign(new AgentTaskControlService(controlRPC!.client, temporalDispatcher!), {
    startTask: (input: { principalUserId: string; requestId?: string; traceId?: string; body: unknown }) => interactiveTaskStarter!.start(input),
    getRuntimeStatus: async () => ({
      schemaVersion: "dipole.agent.runtime_status.v1",
      runtimeMode: shadowConfig.runtimeMode,
      temporal: { enabled: temporalConfig.enabled, activityMode: temporalConfig.activityMode },
      taskControlEnabled: true,
      interactiveMessageWritesEnabled: shadowConfig.runtimeMode === "active" && shadowConfig.interactiveMessageWritesEnabled
    })
  })
  : undefined;
const mcpRegistry = mcpEnabled ? new CapabilityRegistry() : undefined;
if (mcpRegistry !== undefined) mcpRegistry.register(new ConversationListCapability(mcpRPC!.client));
if (mcpRegistry !== undefined) mcpRegistry.register(new ConversationReadCapability(mcpRPC!.client));
const mcpAuthExtraSchema = z.object({
  resource: z.literal(mcpResource),
  taskId: z.string().trim().min(1),
  runId: z.string().trim().min(1),
  requestId: z.string().trim().min(1).optional(),
  traceId: z.string().trim().min(1).optional()
}).strict();
const mcpHandler = mcpRegistry === undefined ? undefined : createDipoleMcpHttpHandler({
  registry: mcpRegistry,
  runner: new McpToolInvocationRunner({
    begin: (input) => mcpRPC!.client.begin(input),
    finish: (input) => mcpRPC!.client.finishToolInvocation(input)
  }, undefined, undefined, undefined, mcpToolTimeoutMs),
  tools: [{
    name: "dipole_conversation_list",
    capabilityId: "conversation.list",
    title: "List conversations",
    description: "List conversations available to the authenticated Dipole Agent Task",
    inputSchema: z.object({ limit: z.number().int().min(1).max(100).default(20) }).strict()
  }],
  resolveContext: (auth) => {
    const binding = mcpAuthExtraSchema.parse(auth.extra);
    return mcpRPC!.client.resolveMcpContext(binding.taskId, binding.runId, auth.clientId, {
      ...(binding.requestId === undefined ? {} : { requestId: binding.requestId }),
      ...(binding.traceId === undefined ? {} : { traceId: binding.traceId })
    });
  }
});
const server = buildServer(
  { isReady: () => ready },
  controlService === undefined ? undefined : { secret: controlSecret, service: controlService },
  mcpHandler === undefined ? undefined : { secret: mcpSecret, resource: mcpResource, handler: mcpHandler },
  subscriptionShadowMetrics
);
const stop = (): Promise<void> => {
  stopPromise ??= (async () => {
    ready = false;
    const failures: unknown[] = [];
    if (externalMcpShadowProcess !== undefined) {
      try {
        await externalMcpShadowProcess.stop();
      } catch (error) {
        failures.push(error);
      }
      externalMcpShadowProcess = undefined;
    }
    if (shadowStarted && shadowRuntime !== undefined) {
      try {
        await shadowRuntime.stop();
      } catch (error) {
        failures.push(error);
      }
      shadowStarted = false;
    }
    if (temporalDispatcherStarted && temporalDispatcher !== undefined) {
      try {
        await temporalDispatcher.stop();
      } catch (error) {
        failures.push(error);
      }
      temporalDispatcherStarted = false;
    }
    if (temporalStarted && temporalRuntime !== undefined) {
      try {
        await temporalRuntime.stop();
      } catch (error) {
        failures.push(error);
      }
      temporalStarted = false;
    }
    if (temporalReadResourcesOpen && temporalReadResources !== undefined) {
      try {
        await temporalReadResources.stop();
      } catch (error) {
        failures.push(error);
      }
      temporalReadResourcesOpen = false;
    }
    temporalRPC?.close();
    temporalRPC = undefined;
    controlRPC?.close();
    if (serverStarted) {
      try {
        await server.close();
      } catch (error) {
        failures.push(error);
      }
      serverStarted = false;
    }
    if (mcpHandler !== undefined) {
      try {
        await mcpHandler.close();
      } catch (error) {
        failures.push(error);
      }
    }
    mcpRPC?.close();
    try {
      await observabilityRuntime.stop();
    } catch (error) {
      failures.push(error);
    }
    if (failures.length > 0) {
      throw new AggregateError(failures, "Agent Runtime shutdown failed");
    }
  })();
  return stopPromise;
};

const onTemporalFailure = (error: unknown): void => {
  if (!ready) return;
  process.stderr.write(`Temporal Worker failed: ${String(error)}\n`);
  process.exitCode = 1;
  void stop().catch((stopError: unknown) => {
    process.stderr.write(`${String(stopError)}\n`);
  });
};

if (temporalConfig.enabled && !externalMcpShadowEnabled) {
  let activities: AgentTaskWorkerActivities = foundationAgentTaskActivities;
  if (temporalConfig.activityMode === "persistent_shadow") {
    if (!shadowConfig.capabilityRpc.enabled) {
      throw new Error("Persistent Temporal shadow Activities require Agent Capability RPC");
    }
    temporalRPC = createAgentCapabilityRPC(shadowConfig);
    activities = {
      ...foundationAgentTaskActivities,
      ...createPersistentAgentTaskLifecycleActivities(temporalRPC.client)
    };
  } else if (temporalConfig.activityMode === "read_shadow" || temporalConfig.activityMode === "read_active" || temporalConfig.activityMode === "interactive_active" || temporalConfig.activityMode === "promotion_active") {
    activities = {
      ...foundationAgentTaskActivities,
      ...createPersistentAgentTaskLifecycleActivities(temporalReadResources!.client),
      ...temporalReadResources!.activities
    };
    if (temporalConfig.activityMode === "promotion_active") {
      activities = {
        ...activities,
        ...createAgentMemoryPromotionCommitActivities(temporalReadResources!.client)
      };
    }
  }
  temporalRuntime = createTemporalWorkerRuntime(
    temporalConfig,
    activities,
    undefined,
    onTemporalFailure
  );
}

try {
  await server.listen({ host, port });
  serverStarted = true;
  if (temporalReadResources !== undefined) {
    await temporalReadResources.start();
  }
  if (temporalRuntime !== undefined) {
    temporalStarted = true;
    await temporalRuntime.start();
  }
  if (externalMcpShadowEnabled) {
    externalMcpShadowProcess = await startExternalMcpProductionShadow(
      externalMcpEnvironment,
      shadowConfig,
      temporalConfig,
      foundationAgentTaskActivities,
      {},
      onTemporalFailure
    );
  }
  if (temporalDispatcher !== undefined) {
    temporalDispatcherStarted = true;
    await temporalDispatcher.start();
  }
  if (shadowRuntime !== undefined) {
    shadowStarted = true;
    await shadowRuntime.start();
  }
  ready = true;
} catch (error) {
  process.stderr.write(`${String(error)}\n`);
  try {
    await stop();
  } catch (stopError) {
    process.stderr.write(`${String(stopError)}\n`);
  }
  process.exitCode = 1;
}

for (const signal of ["SIGINT", "SIGTERM"] as const) {
  process.once(signal, () => {
    void stop().catch((error: unknown) => {
      process.stderr.write(`${String(error)}\n`);
      process.exitCode = 1;
    });
  });
}
