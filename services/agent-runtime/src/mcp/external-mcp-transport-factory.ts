import {
  StreamableHTTPClientTransport,
  type StreamableHTTPClientTransportOptions,
  type Transport
} from "@modelcontextprotocol/client";

import {
  createExternalMcpAuthProvider,
  type ExternalMcpAuthProviderOptions,
  type ExternalMcpSecretProvider
} from "./external-mcp-auth-provider.js";
import type { ExternalMcpCredentialBinding } from "./external-mcp-credential-catalog.js";
import {
  createExternalMcpNetworkGuardedFetch,
  type ExternalMcpDnsResolver,
  type ExternalMcpNetworkDispatcher
} from "./external-mcp-network-guard.js";
import type { ExternalMcpProfile, ExternalMcpTransportFactory } from "./external-mcp-profile.js";

export type ExternalMcpTransportFactoryErrorCode =
  | "setup_cancelled"
  | "profile_invalid"
  | "credential_binding_mismatch"
  | "transport_unavailable";

export class ExternalMcpTransportFactoryError extends Error {
  readonly code: ExternalMcpTransportFactoryErrorCode;

  constructor(code: ExternalMcpTransportFactoryErrorCode) {
    super({
      setup_cancelled: "External MCP transport setup was cancelled",
      profile_invalid: "External MCP transport profile is invalid",
      credential_binding_mismatch: "External MCP credential binding does not match its profile",
      transport_unavailable: "External MCP transport is unavailable"
    }[code]);
    this.name = "ExternalMcpTransportFactoryError";
    this.code = code;
  }
}

export interface ExternalMcpStreamableHttpTransportBuilder {
  create(url: URL, options: StreamableHTTPClientTransportOptions): Transport;
}

export interface ExternalMcpStreamableHttpTransportFactoryDependencies {
  readonly secretProvider: ExternalMcpSecretProvider;
  readonly resolver: ExternalMcpDnsResolver;
  readonly dispatcher: ExternalMcpNetworkDispatcher;
  readonly authProviderOptions?: ExternalMcpAuthProviderOptions;
  readonly transportBuilder?: ExternalMcpStreamableHttpTransportBuilder;
}

const defaultTransportBuilder: ExternalMcpStreamableHttpTransportBuilder = {
  create: (url, options) => new StreamableHTTPClientTransport(url, options)
};

export function createExternalMcpStreamableHttpTransportFactory(
  dependencies: ExternalMcpStreamableHttpTransportFactoryDependencies
): ExternalMcpTransportFactory {
  const transportBuilder = dependencies.transportBuilder ?? defaultTransportBuilder;
  return {
    async connect({ profile, credential }, signal) {
      if (signal?.aborted) throw new ExternalMcpTransportFactoryError("setup_cancelled");
      assertCredentialBinding(profile, credential);
      const endpoint = parseEndpoint(profile);
      const authProvider = createExternalMcpAuthProvider(
        credential,
        dependencies.secretProvider,
        dependencies.authProviderOptions
      );
      const guardedFetch = createExternalMcpNetworkGuardedFetch(profile, dependencies.resolver, dependencies.dispatcher);
      if (signal?.aborted) throw new ExternalMcpTransportFactoryError("setup_cancelled");

      try {
        return transportBuilder.create(endpoint, {
          authProvider,
          fetch: guardedFetch,
          requestInit: { redirect: "manual" },
          onInsufficientScope: "throw",
          maxStepUpRetries: 0,
          reconnectionOptions: {
            initialReconnectionDelay: 1000,
            maxReconnectionDelay: 1000,
            reconnectionDelayGrowFactor: 1,
            maxRetries: 0
          }
        });
      } catch {
        throw new ExternalMcpTransportFactoryError("transport_unavailable");
      }
    }
  };
}

function assertCredentialBinding(profile: ExternalMcpProfile, credential: ExternalMcpCredentialBinding): void {
  if (
    credential.tenantId !== profile.tenantId
    || credential.credentialRef !== profile.credentialRef
    || credential.credentialVersion !== profile.credentialVersion
  ) {
    throw new ExternalMcpTransportFactoryError("credential_binding_mismatch");
  }
}

function parseEndpoint(profile: ExternalMcpProfile): URL {
  try {
    const endpoint = new URL(profile.endpoint);
    const port = endpoint.port === "" ? 443 : Number(endpoint.port);
    if (
      profile.dnsResolution !== "public_only"
      || endpoint.protocol !== "https:"
      || endpoint.username !== ""
      || endpoint.password !== ""
      || endpoint.search !== ""
      || endpoint.hash !== ""
      || endpoint.hostname !== profile.tlsServerName
      || !profile.allowedHosts.includes(endpoint.hostname)
      || !profile.allowedPorts.includes(port)
    ) {
      throw new Error("invalid profile");
    }
    return endpoint;
  } catch {
    throw new ExternalMcpTransportFactoryError("profile_invalid");
  }
}
