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
  readonly transportBuilder?: ExternalMcpStreamableHttpTransportBuilder;
  readonly now?: () => Date;
}

export function createExternalMcpProductionIoRegistry(
  config: ExternalMcpConfig,
  io?: ExternalMcpProductionIoConfig,
  options: ExternalMcpProductionIoOptions = {}
): ExternalMcpTransportRegistry {
  if (!config.enabled) {
    return new ExternalMcpTransportRegistry(config, disabledCatalog, disabledFactory, options.now);
  }
  if (io === undefined) throw new Error("Enabled external MCP requires production I/O configuration");

  const owner = options.expectedOwnerUid === undefined ? {} : { expectedOwnerUid: options.expectedOwnerUid };
  const catalog = createFileExternalMcpCredentialCatalog(io.credentialCatalogPath, {
    ...owner,
    ...(options.maximumCatalogBytes === undefined ? {} : { maximumBytes: options.maximumCatalogBytes })
  });
  const secretProvider = createEncryptedFileExternalMcpSecretProvider(io.secretProvider, {
    ...owner,
    ...(options.maximumSecretBytes === undefined ? {} : { maximumSecretBytes: options.maximumSecretBytes })
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
    ...(options.transportBuilder === undefined ? {} : { transportBuilder: options.transportBuilder })
  });
  return new ExternalMcpTransportRegistry(config, catalog, factory, options.now);
}

const disabledCatalog: ExternalMcpCredentialCatalog = {
  resolve: async () => { throw new Error("External MCP connections are disabled"); }
};

const disabledFactory: ExternalMcpTransportFactory = {
  connect: async (): Promise<Transport> => { throw new Error("External MCP connections are disabled"); }
};
