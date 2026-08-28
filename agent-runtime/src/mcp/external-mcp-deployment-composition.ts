import type { TemporalMcpDispatchRoute } from "../temporal/mcp-dispatch-activity.js";
import type { McpWorkerExternalMcpDependencies } from "./mcp-worker-runtime.js";
import {
  type ExternalMcpCapabilityDefinitionRegistry,
  type LoadedExternalMcpDeploymentRoutes,
  loadExternalMcpDeploymentRouteManifest
} from "./external-mcp-deployment-route-manifest.js";
import {
  loadExternalMcpConfig,
  type ExternalMcpConfig
} from "./external-mcp-profile.js";
import {
  loadExternalMcpProductionIoManifest
} from "./external-mcp-production-io-manifest.js";
import {
  createExternalMcpProductionIoRuntime,
  type ExternalMcpProductionIoRuntime
} from "./external-mcp-production-io.js";
import {
  externalMcpReadinessBindingSha256,
  type ExternalMcpReadinessBindingOptions
} from "./external-mcp-readiness-evidence.js";

export interface ExternalMcpDeploymentPlanOptions {
  readonly expectedOwnerUid?: number;
  readonly maximumIoManifestBytes?: number;
  readonly maximumRouteManifestBytes?: number;
  readonly maximumReadinessCollectionMs?: number;
  readonly now?: () => Date;
  readonly signal?: AbortSignal;
}

export interface ExternalMcpDeploymentPlan {
  readonly config: Extract<ExternalMcpConfig, { enabled: true }>;
  readonly productionIo: ExternalMcpProductionIoRuntime;
  readonly routeRegistry: LoadedExternalMcpDeploymentRoutes["registry"];
  readonly routes: readonly TemporalMcpDispatchRoute[];
  readonly workerExternalMcp: McpWorkerExternalMcpDependencies;
  readonly runtimeBindingSha256: string;
}

export async function loadExternalMcpDeploymentPlan(
  definitions: ExternalMcpCapabilityDefinitionRegistry,
  env: NodeJS.ProcessEnv,
  options: ExternalMcpDeploymentPlanOptions = {}
): Promise<ExternalMcpDeploymentPlan | undefined> {
  const signal = options.signal ?? new AbortController().signal;
  signal.throwIfAborted();
  try {
    const config = loadExternalMcpConfig(env);
    if (!config.enabled) return undefined;
    const expectedOwnerUid = options.expectedOwnerUid ?? process.getuid?.();
    if (expectedOwnerUid === undefined || !Number.isSafeInteger(expectedOwnerUid) || expectedOwnerUid < 0) {
      throw new Error("invalid owner");
    }
    const loadedIo = await loadExternalMcpProductionIoManifest(config, env, {
      expectedOwnerUid,
      ...(options.maximumIoManifestBytes === undefined ? {} : { maximumManifestBytes: options.maximumIoManifestBytes }),
      signal
    });
    signal.throwIfAborted();
    if (loadedIo === undefined) throw new Error("missing production I/O");
    const loadedRoutes = await loadExternalMcpDeploymentRouteManifest(config, definitions, env, {
      expectedOwnerUid,
      ...(options.maximumRouteManifestBytes === undefined ? {} : { maximumManifestBytes: options.maximumRouteManifestBytes }),
      signal
    });
    signal.throwIfAborted();
    if (loadedRoutes === undefined) throw new Error("missing deployment routes");

    const bindingOptions = readinessBindingOptions(loadedIo.options);
    const productionIo = createExternalMcpProductionIoRuntime(config, loadedIo.io, {
      ...loadedIo.options,
      ...(options.maximumReadinessCollectionMs === undefined ? {} : {
        maximumReadinessCollectionMs: options.maximumReadinessCollectionMs
      }),
      ...(options.now === undefined ? {} : { now: options.now })
    });
    const workerExternalMcp: McpWorkerExternalMcpDependencies = {
      config,
      io: loadedIo.io,
      registry: productionIo.registry,
      readinessBindingOptions: bindingOptions
    };
    return {
      config,
      productionIo,
      routeRegistry: loadedRoutes.registry,
      routes: loadedRoutes.routes,
      workerExternalMcp,
      runtimeBindingSha256: externalMcpReadinessBindingSha256(config, loadedIo.io, bindingOptions)
    };
  } catch {
    if (signal.aborted) signal.throwIfAborted();
    throw new Error("External MCP deployment plan is unavailable");
  }
}

function readinessBindingOptions(
  options: {
    readonly expectedOwnerUid?: number;
    readonly maximumCatalogBytes?: number;
    readonly maximumSecretBytes?: number;
    readonly maximumCaBundleBytes?: number;
    readonly connectTimeoutMs?: number;
  }
): ExternalMcpReadinessBindingOptions {
  if (options.expectedOwnerUid === undefined || options.maximumCatalogBytes === undefined ||
      options.maximumSecretBytes === undefined || options.maximumCaBundleBytes === undefined ||
      options.connectTimeoutMs === undefined) {
    throw new Error("production I/O options are incomplete");
  }
  return {
    expectedOwnerUid: options.expectedOwnerUid,
    maximumCatalogBytes: options.maximumCatalogBytes,
    maximumSecretBytes: options.maximumSecretBytes,
    maximumCaBundleBytes: options.maximumCaBundleBytes,
    connectTimeoutMs: options.connectTimeoutMs,
    trustedTransportBuilder: true
  };
}
