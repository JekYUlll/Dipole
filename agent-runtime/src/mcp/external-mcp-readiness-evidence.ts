import { createHash } from "node:crypto";

import { canonicalMcpJSON } from "./canonical-json.js";
import type { ExternalMcpConfig, ExternalMcpProfile } from "./external-mcp-profile.js";
import type {
  ExternalMcpProductionIoConfig
} from "./external-mcp-production-io.js";
import type {
  ExternalMcpProductionIoPreflight,
  ExternalMcpProductionIoPreflightReceipt
} from "./external-mcp-production-io-preflight.js";
import type {
  ExternalMcpShadowConnectivityDrill,
  ExternalMcpShadowConnectivityReceipt
} from "./external-mcp-shadow-connectivity.js";

export const externalMcpReadinessEvidenceSchemaVersion =
  "dipole.agent.external-mcp-readiness-evidence.v1" as const;
const bindingSchemaVersion = "dipole.agent.external-mcp-readiness-binding.v1" as const;

export interface ExternalMcpReadinessBindingOptions {
  readonly expectedOwnerUid?: number;
  readonly maximumCatalogBytes?: number;
  readonly maximumSecretBytes?: number;
  readonly maximumCaBundleBytes?: number;
  readonly connectTimeoutMs?: number;
  readonly trustedTransportBuilder?: boolean;
}

export interface ExternalMcpReadinessEvidenceCollectorOptions extends ExternalMcpReadinessBindingOptions {
  readonly maximumCollectionMs?: number;
  readonly now?: () => Date;
}

export interface ExternalMcpReadinessEvidenceDependencies {
  readonly preflight: ExternalMcpProductionIoPreflight;
  readonly shadowConnectivityDrill: ExternalMcpShadowConnectivityDrill;
}

export interface ExternalMcpReadinessEvidence {
  readonly schemaVersion: typeof externalMcpReadinessEvidenceSchemaVersion;
  readonly bindingSha256: string;
  readonly startedAt: string;
  readonly completedAt: string;
  readonly preflightCheckedAt: string;
  readonly connectivityCheckedAt: string;
  readonly profileCount: number;
  readonly credentialCount: number;
  readonly caBundleCount: number;
  readonly toolCount: number;
}

export type ExternalMcpReadinessEvidenceCollector = (
  input: { readonly profileId: string; readonly tenantId: string },
  signal?: AbortSignal
) => Promise<ExternalMcpReadinessEvidence>;

export function createExternalMcpReadinessEvidenceCollector(
  config: ExternalMcpConfig,
  io: ExternalMcpProductionIoConfig,
  dependencies: ExternalMcpReadinessEvidenceDependencies,
  options: ExternalMcpReadinessEvidenceCollectorOptions = {}
): ExternalMcpReadinessEvidenceCollector {
  const maximumCollectionMs = options.maximumCollectionMs ?? 300_000;
  if (!Number.isSafeInteger(maximumCollectionMs) || maximumCollectionMs < 100 || maximumCollectionMs > 600_000) {
    throw new Error("External MCP readiness evidence collection window is invalid");
  }
  const now = options.now ?? (() => new Date());
  const trustedTransportBuilder = options.trustedTransportBuilder ?? true;
  const bindingSha256 = externalMcpReadinessBindingSha256(config, io, options);

  return async (input, signal = new AbortController().signal) => {
    signal.throwIfAborted();
    try {
      if (!trustedTransportBuilder || !config.enabled) throw new Error("untrusted readiness dependencies");
      const profile = config.profiles.find(candidate =>
        candidate.profileId === input.profileId && candidate.tenantId === input.tenantId
      );
      if (profile === undefined) throw new Error("unknown readiness Profile");
      const startedAt = validDate(now());
      const preflight = await dependencies.preflight(signal);
      signal.throwIfAborted();
      const connectivity = await dependencies.shadowConnectivityDrill(input, signal);
      signal.throwIfAborted();
      const completedAt = validDate(now());
      assertReceipts(config.profiles.length, profile, preflight, connectivity, startedAt, completedAt, maximumCollectionMs);
      return {
        schemaVersion: externalMcpReadinessEvidenceSchemaVersion,
        bindingSha256,
        startedAt: startedAt.toISOString(),
        completedAt: completedAt.toISOString(),
        preflightCheckedAt: preflight.checkedAt,
        connectivityCheckedAt: connectivity.checkedAt,
        profileCount: preflight.profileCount,
        credentialCount: preflight.credentialCount,
        caBundleCount: preflight.caBundleCount,
        toolCount: connectivity.toolCount
      };
    } catch {
      if (signal.aborted) signal.throwIfAborted();
      throw new Error("External MCP readiness evidence failed");
    }
  };
}

export function externalMcpReadinessBindingSha256(
  config: ExternalMcpConfig,
  io: ExternalMcpProductionIoConfig,
  options: ExternalMcpReadinessBindingOptions = {}
): string {
  if (!config.enabled) throw new Error("External MCP readiness binding requires enabled Profiles");
  const expectedOwnerUid = options.expectedOwnerUid ?? process.getuid?.();
  if (expectedOwnerUid === undefined || !Number.isSafeInteger(expectedOwnerUid) || expectedOwnerUid < 0) {
    throw new Error("External MCP readiness binding requires an expected owner UID");
  }
  const binding = {
    schema_version: bindingSchemaVersion,
    profiles: [...config.profiles].sort(compareProfile).map(profile => ({
      profile_id: profile.profileId,
      tenant_id: profile.tenantId,
      server_id: profile.serverId,
      endpoint: profile.endpoint,
      credential_ref: profile.credentialRef,
      credential_version: profile.credentialVersion,
      allowed_hosts: [...profile.allowedHosts].sort(),
      allowed_ports: [...profile.allowedPorts].sort((left, right) => left - right),
      dns_resolution: profile.dnsResolution,
      tls_server_name: profile.tlsServerName,
      ca_bundle_ref: profile.caBundleRef,
      allowed_tools: [...profile.allowedTools].sort()
    })),
    production_io: {
      credential_catalog_path: io.credentialCatalogPath,
      provider_id: io.secretProvider.providerId,
      keys: sortedEntries(io.secretProvider.keys).map(([keyRef, path]) => ({ key_ref: keyRef, path })),
      secrets: sortedEntries(io.secretProvider.secrets).map(([secretRef, secret]) => ({
        secret_ref: secretRef,
        key_ref: secret.keyRef,
        path: secret.path
      })),
      ca_bundles: sortedEntries(io.caBundles).map(([caBundleRef, path]) => ({ ca_bundle_ref: caBundleRef, path }))
    },
    effective_options: {
      expected_owner_uid: expectedOwnerUid,
      maximum_catalog_bytes: options.maximumCatalogBytes ?? 256 * 1024,
      maximum_secret_bytes: options.maximumSecretBytes ?? 4096,
      maximum_ca_bundle_bytes: options.maximumCaBundleBytes ?? 256 * 1024,
      connect_timeout_ms: options.connectTimeoutMs ?? 5_000,
      dns_timeout_ms: 2_000,
      secret_timeout_ms: 2_000,
      shadow_request_timeout_ms: 10_000,
      trusted_transport_builder: options.trustedTransportBuilder ?? true
    }
  };
  return createHash("sha256")
    .update(`${bindingSchemaVersion}\n${canonicalMcpJSON(binding)}`, "utf8")
    .digest("hex");
}

function assertReceipts(
  expectedProfileCount: number,
  profile: ExternalMcpProfile,
  preflight: ExternalMcpProductionIoPreflightReceipt,
  connectivity: ExternalMcpShadowConnectivityReceipt,
  startedAt: Date,
  completedAt: Date,
  maximumCollectionMs: number
): void {
  const preflightAt = validDate(new Date(preflight.checkedAt));
  const connectivityAt = validDate(new Date(connectivity.checkedAt));
  if (preflight.schemaVersion !== "dipole.agent.external-mcp-production-io-preflight.v1" || !preflight.enabled ||
      preflight.profileCount !== expectedProfileCount || preflight.credentialCount < 1 ||
      preflight.credentialCount > expectedProfileCount || preflight.caBundleCount < 1 ||
      preflight.caBundleCount > expectedProfileCount ||
      connectivity.schemaVersion !== "dipole.agent.external-mcp-shadow-connectivity.v1" ||
      connectivity.toolCount !== profile.allowedTools.length ||
      completedAt.getTime() < startedAt.getTime() || completedAt.getTime() - startedAt.getTime() > maximumCollectionMs ||
      preflightAt.getTime() < startedAt.getTime() || preflightAt.getTime() > completedAt.getTime() ||
      connectivityAt.getTime() < preflightAt.getTime() || connectivityAt.getTime() > completedAt.getTime()) {
    throw new Error("External MCP readiness evidence receipts are inconsistent");
  }
}

function validDate(value: Date): Date {
  if (!Number.isFinite(value.getTime())) throw new Error("External MCP readiness evidence time is invalid");
  return value;
}

function compareProfile(left: ExternalMcpProfile, right: ExternalMcpProfile): number {
  const leftKey = `${left.tenantId}\0${left.profileId}`;
  const rightKey = `${right.tenantId}\0${right.profileId}`;
  return leftKey < rightKey ? -1 : leftKey > rightKey ? 1 : 0;
}

function sortedEntries<T>(value: Readonly<Record<string, T>>): Array<[string, T]> {
  return Object.entries(value).sort(([left], [right]) => left < right ? -1 : left > right ? 1 : 0);
}
