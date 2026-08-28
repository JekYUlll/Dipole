import type { McpActivityExternalTransportRegistry, McpActivityModernClientFactory } from "../mcp/mcp-input-required-activity.js";
import type { McpInvocationBeginClient } from "../mcp/mcp-invocation-producer.js";
import { ExternalMcpCapabilityRouteRegistry, TrustedMcpInvocationProducer } from "../mcp/mcp-invocation-producer.js";
import type { ExternalMcpArtifactCommandResolver, ExternalMcpArtifactWriter } from "../mcp/external-mcp-artifact-projector.js";
import { ExternalMcpArtifactProjector } from "../mcp/external-mcp-artifact-projector.js";
import type { McpInvocationTerminalClient } from "../mcp/mcp-terminal-worker-runtime.js";
import type { McpWorkerCoreClient } from "../mcp/mcp-worker-runtime.js";
import { createMcpTerminalWorkerRuntime } from "../mcp/mcp-worker-runtime.js";
import {
  createTemporalMcpDispatchActivities,
  temporalMcpDispatchRouteBinding,
  type TemporalMcpContextResolver,
  type TemporalMcpDispatchActivities,
  type TemporalMcpDispatchRoute,
  type TemporalMcpDispatchRouteBinding
} from "./mcp-dispatch-activity.js";

export interface TemporalMcpDispatchRuntimeCore extends
  TemporalMcpContextResolver,
  McpInvocationBeginClient,
  McpWorkerCoreClient,
  McpInvocationTerminalClient,
  ExternalMcpArtifactCommandResolver {}

export interface TemporalMcpDispatchRuntimeDependencies {
  readonly routes: ExternalMcpCapabilityRouteRegistry;
  readonly core: TemporalMcpDispatchRuntimeCore;
  readonly transports: McpActivityExternalTransportRegistry;
  readonly artifacts: ExternalMcpArtifactWriter;
  readonly requestTimeoutMs?: number;
  readonly inputWindowMs?: number;
  readonly now?: () => number;
  readonly ownerTokenSha256?: () => string;
  readonly createClient?: McpActivityModernClientFactory;
  readonly cancellationSignal?: () => AbortSignal;
}

export interface TemporalMcpDispatchRuntime {
  readonly routeBinding: TemporalMcpDispatchRouteBinding;
  readonly activities: TemporalMcpDispatchActivities;
}

export function createTemporalMcpDispatchRuntime(
  route: TemporalMcpDispatchRoute,
  dependencies: TemporalMcpDispatchRuntimeDependencies
): TemporalMcpDispatchRuntime {
  const worker = createMcpTerminalWorkerRuntime({
    core: dependencies.core,
    transports: dependencies.transports,
    egressPolicies: dependencies.routes.workerEgressPolicies(route.capabilityId),
    ...(dependencies.requestTimeoutMs === undefined ? {} : { requestTimeoutMs: dependencies.requestTimeoutMs }),
    ...(dependencies.inputWindowMs === undefined ? {} : { inputWindowMs: dependencies.inputWindowMs }),
    ...(dependencies.now === undefined ? {} : { now: dependencies.now }),
    ...(dependencies.ownerTokenSha256 === undefined ? {} : { ownerTokenSha256: dependencies.ownerTokenSha256 }),
    ...(dependencies.createClient === undefined ? {} : { createClient: dependencies.createClient })
  });
  const activities = createTemporalMcpDispatchActivities(route, {
    contexts: dependencies.core,
    producer: new TrustedMcpInvocationProducer(dependencies.routes, dependencies.core),
    worker,
    projector: new ExternalMcpArtifactProjector(dependencies.core, dependencies.artifacts),
    ...(dependencies.cancellationSignal === undefined ? {} : { cancellationSignal: dependencies.cancellationSignal })
  });
  return { routeBinding: temporalMcpDispatchRouteBinding(route), activities };
}
