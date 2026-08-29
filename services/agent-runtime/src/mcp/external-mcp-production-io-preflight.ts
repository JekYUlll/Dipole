import { createExternalMcpAuthProvider, type ExternalMcpSecretProvider } from "./external-mcp-auth-provider.js";
import type { ExternalMcpCredentialBinding, ExternalMcpCredentialCatalog } from "./external-mcp-credential-catalog.js";
import type { ExternalMcpConfig, ExternalMcpProfile } from "./external-mcp-profile.js";
import type { ExternalMcpCaBundleProvider } from "./node-external-mcp-ca-bundle-provider.js";

export const externalMcpProductionIoPreflightSchemaVersion = "dipole.agent.external-mcp-production-io-preflight.v1" as const;

export interface ExternalMcpProductionIoPreflightDependencies {
  readonly catalog: ExternalMcpCredentialCatalog;
  readonly secretProvider: ExternalMcpSecretProvider;
  readonly caBundles: ExternalMcpCaBundleProvider;
  readonly maximumSecretBytes: number;
}

export interface ExternalMcpProductionIoPreflightReceipt {
  readonly schemaVersion: typeof externalMcpProductionIoPreflightSchemaVersion;
  readonly enabled: boolean;
  readonly checkedAt: string;
  readonly profileCount: number;
  readonly credentialCount: number;
  readonly caBundleCount: number;
}

export type ExternalMcpProductionIoPreflight = (
  signal?: AbortSignal
) => Promise<ExternalMcpProductionIoPreflightReceipt>;

export function createExternalMcpProductionIoPreflight(
  config: ExternalMcpConfig,
  dependencies: ExternalMcpProductionIoPreflightDependencies | undefined,
  now: () => Date = () => new Date()
): ExternalMcpProductionIoPreflight {
  const profiles = config.enabled ? config.profiles.map(profile => copyProfile(profile)) : [];
  if (config.enabled && dependencies === undefined) {
    throw new Error("Enabled external MCP preflight requires production I/O dependencies");
  }
  return async (signal = new AbortController().signal) => {
    signal.throwIfAborted();
    try {
      const checkedAt = now();
      if (!Number.isFinite(checkedAt.getTime())) throw new Error("invalid readiness clock");
      if (!config.enabled) return receipt(false, checkedAt, 0, 0, 0);

      const credentials = new Map<string, ExternalMcpCredentialBinding>();
      const caRefs = new Set<string>();
      for (const profile of profiles) {
        signal.throwIfAborted();
        const credential = await dependencies!.catalog.resolve({
          tenantId: profile.tenantId,
          credentialRef: profile.credentialRef,
          credentialVersion: profile.credentialVersion,
          now: checkedAt
        });
        signal.throwIfAborted();
        assertResolvedBinding(profile, credential);
        credentials.set(credentialKey(credential), { ...credential });
        caRefs.add(profile.caBundleRef);
      }
      for (const credential of credentials.values()) {
        signal.throwIfAborted();
        await createExternalMcpAuthProvider(credential, dependencies!.secretProvider, {
          maximumBytes: dependencies!.maximumSecretBytes
        }).token();
        signal.throwIfAborted();
      }
      for (const caRef of caRefs) {
        signal.throwIfAborted();
        await dependencies!.caBundles.read(caRef, signal);
        signal.throwIfAborted();
      }
      return receipt(true, checkedAt, profiles.length, credentials.size, caRefs.size);
    } catch {
      if (signal.aborted) signal.throwIfAborted();
      throw new Error("External MCP production I/O preflight failed");
    }
  };
}

function assertResolvedBinding(profile: ExternalMcpProfile, credential: ExternalMcpCredentialBinding): void {
  if (credential.tenantId !== profile.tenantId || credential.credentialRef !== profile.credentialRef ||
      credential.credentialVersion !== profile.credentialVersion) {
    throw new Error("Catalog returned a mismatched binding");
  }
}

function credentialKey(credential: ExternalMcpCredentialBinding): string {
  return [
    credential.tenantId,
    credential.credentialRef,
    String(credential.credentialVersion),
    credential.providerId,
    credential.providerSecretRef
  ].join("\0");
}

function receipt(
  enabled: boolean,
  checkedAt: Date,
  profileCount: number,
  credentialCount: number,
  caBundleCount: number
): ExternalMcpProductionIoPreflightReceipt {
  return {
    schemaVersion: externalMcpProductionIoPreflightSchemaVersion,
    enabled,
    checkedAt: checkedAt.toISOString(),
    profileCount,
    credentialCount,
    caBundleCount
  };
}

function copyProfile(profile: ExternalMcpProfile): ExternalMcpProfile {
  return {
    ...profile,
    allowedHosts: [...profile.allowedHosts],
    allowedPorts: [...profile.allowedPorts],
    allowedTools: [...profile.allowedTools]
  };
}
