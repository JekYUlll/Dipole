import type { Transport } from "@modelcontextprotocol/client";

import {
  createFileExternalMcpCredentialCatalog,
  type ExternalMcpCredentialCatalog
} from "./external-mcp-credential-catalog.js";
import {
  ExternalMcpTransportRegistry,
  type ExternalMcpConfig,
  type ExternalMcpTransportFactory
} from "./external-mcp-profile.js";
import {
  createExternalMcpStreamableHttpTransportFactory,
  type ExternalMcpStreamableHttpTransportBuilder
} from "./external-mcp-transport-factory.js";
import {
  createExternalMcpProductionIoPreflight,
  type ExternalMcpProductionIoPreflight
} from "./external-mcp-production-io-preflight.js";
import {
  createExternalMcpReadinessEvidenceCollector,
  type ExternalMcpReadinessEvidenceCollector
} from "./external-mcp-readiness-evidence.js";
import {
  createExternalMcpShadowConnectivityDrill,
  type ExternalMcpShadowConnectivityDrill
} from "./external-mcp-shadow-connectivity.js";
import {
  createEncryptedFileExternalMcpSecretProvider,
  type EncryptedFileExternalMcpSecretProviderConfig
} from "./node-external-mcp-encrypted-secret-provider.js";
import { createFileExternalMcpCaBundleProvider } from "./node-external-mcp-ca-bundle-provider.js";
import { NodeExternalMcpDnsResolver } from "./node-external-mcp-dns-resolver.js";
import { NodeExternalMcpPinnedTlsDispatcher } from "./node-external-mcp-pinned-tls-dispatcher.js";

export interface ExternalMcpProductionIoConfig {
  readonly credentialCatalogPath: string;
  readonly secretProvider: EncryptedFileExternalMcpSecretProviderConfig;
  readonly caBundles: Readonly<Record<string, string>>;
}

export interface ExternalMcpProductionIoOptions {
  readonly expectedOwnerUid?: number;
  readonly maximumCatalogBytes?: number;
  readonly maximumSecretBytes?: number;
  readonly maximumCaBundleBytes?: number;
  readonly connectTimeoutMs?: number;
  readonly maximumReadinessCollectionMs?: number;
  readonly transportBuilder?: ExternalMcpStreamableHttpTransportBuilder;
  readonly now?: () => Date;
}

export interface ExternalMcpProductionIoRuntime {
  readonly registry: ExternalMcpTransportRegistry;
  readonly preflight: ExternalMcpProductionIoPreflight;
  readonly shadowConnectivityDrill: ExternalMcpShadowConnectivityDrill;
  readonly readinessEvidence: ExternalMcpReadinessEvidenceCollector;
}

export function createExternalMcpProductionIoRegistry(
  config: ExternalMcpConfig,
  io?: ExternalMcpProductionIoConfig,
  options: ExternalMcpProductionIoOptions = {}
): ExternalMcpTransportRegistry {
  return createExternalMcpProductionIoRuntime(config, io, options).registry;
}

export function createExternalMcpProductionIoRuntime(
  config: ExternalMcpConfig,
  io?: ExternalMcpProductionIoConfig,
  options: ExternalMcpProductionIoOptions = {}
): ExternalMcpProductionIoRuntime {
  if (!config.enabled) {
    const registry = new ExternalMcpTransportRegistry(config, disabledCatalog, disabledFactory, options.now);
    return {
      registry,
      preflight: createExternalMcpProductionIoPreflight(config, undefined, options.now),
      shadowConnectivityDrill: createExternalMcpShadowConnectivityDrill(registry, {
        ...(options.now === undefined ? {} : { now: options.now })
      }),
      readinessEvidence: disabledReadinessEvidence
    };
  }
  if (io === undefined) throw new Error("Enabled external MCP requires production I/O configuration");

  const maximumSecretBytes = options.maximumSecretBytes ?? 4096;
  const owner = options.expectedOwnerUid === undefined ? {} : { expectedOwnerUid: options.expectedOwnerUid };
  const catalog = createFileExternalMcpCredentialCatalog(io.credentialCatalogPath, {
    ...owner,
    ...(options.maximumCatalogBytes === undefined ? {} : { maximumBytes: options.maximumCatalogBytes })
  });
  const secretProvider = createEncryptedFileExternalMcpSecretProvider(io.secretProvider, {
    ...owner,
    maximumSecretBytes
  });
  const caBundles = createFileExternalMcpCaBundleProvider(io.caBundles, {
    ...owner,
    ...(options.maximumCaBundleBytes === undefined ? {} : { maximumBytes: options.maximumCaBundleBytes })
  });
  const resolver = new NodeExternalMcpDnsResolver();
  const dispatcher = new NodeExternalMcpPinnedTlsDispatcher(caBundles, {
    ...(options.connectTimeoutMs === undefined ? {} : { connectTimeoutMs: options.connectTimeoutMs })
  });
  const factory = createExternalMcpStreamableHttpTransportFactory({
    secretProvider,
    resolver,
    dispatcher,
    authProviderOptions: { maximumBytes: maximumSecretBytes },
    ...(options.transportBuilder === undefined ? {} : { transportBuilder: options.transportBuilder })
  });
  const registry = new ExternalMcpTransportRegistry(config, catalog, factory, options.now);
  const preflight = createExternalMcpProductionIoPreflight(config, {
    catalog,
    secretProvider,
    caBundles,
    maximumSecretBytes
  }, options.now);
  const shadowConnectivityDrill = createExternalMcpShadowConnectivityDrill(registry, {
    ...(options.now === undefined ? {} : { now: options.now })
  });
  return {
    registry,
    preflight,
    shadowConnectivityDrill,
    readinessEvidence: createExternalMcpReadinessEvidenceCollector(config, io, {
      preflight,
      shadowConnectivityDrill
    }, {
      ...(options.expectedOwnerUid === undefined ? {} : { expectedOwnerUid: options.expectedOwnerUid }),
      ...(options.maximumCatalogBytes === undefined ? {} : { maximumCatalogBytes: options.maximumCatalogBytes }),
      maximumSecretBytes,
      ...(options.maximumCaBundleBytes === undefined ? {} : { maximumCaBundleBytes: options.maximumCaBundleBytes }),
      ...(options.connectTimeoutMs === undefined ? {} : { connectTimeoutMs: options.connectTimeoutMs }),
      ...(options.maximumReadinessCollectionMs === undefined ? {} : {
        maximumCollectionMs: options.maximumReadinessCollectionMs
      }),
      trustedTransportBuilder: options.transportBuilder === undefined,
      ...(options.now === undefined ? {} : { now: options.now })
    })
  };
}

const disabledCatalog: ExternalMcpCredentialCatalog = {
  resolve: async () => { throw new Error("External MCP connections are disabled"); }
};

const disabledFactory: ExternalMcpTransportFactory = {
  connect: async (): Promise<Transport> => { throw new Error("External MCP connections are disabled"); }
};

const disabledReadinessEvidence: ExternalMcpReadinessEvidenceCollector = async (_input, signal) => {
  signal?.throwIfAborted();
  throw new Error("External MCP readiness evidence failed");
};
