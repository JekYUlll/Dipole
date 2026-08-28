import { readFile } from "node:fs/promises";
import { describe, expect, it, vi } from "vitest";

import type { ExternalMcpConfig } from "./external-mcp-profile.js";
import type { ExternalMcpProductionIoConfig } from "./external-mcp-production-io.js";
import {
  createExternalMcpReadinessEvidenceCollector,
  externalMcpReadinessBindingSha256,
  externalMcpReadinessEvidenceSchemaVersion
} from "./external-mcp-readiness-evidence.js";

describe("external MCP readiness evidence", () => {
  it("keeps the language-neutral evidence contract aligned with Runtime output", async () => {
    const path = new URL("../../../contracts/agent-external-mcp/v1/readiness-evidence.schema.json", import.meta.url);
    const schema = JSON.parse(await readFile(path, "utf8")) as {
      $id: string;
      "x-dipole-version": string;
      required: string[];
      properties: { schemaVersion: { const: string } };
    };
    expect(schema.$id).toMatch(/agent-external-mcp\/v1\/readiness-evidence\.schema\.json$/);
    expect(schema["x-dipole-version"]).toBe(externalMcpReadinessEvidenceSchemaVersion);
    expect(schema.properties.schemaVersion.const).toBe(externalMcpReadinessEvidenceSchemaVersion);
    expect(schema.required.sort()).toEqual([
      "bindingSha256", "caBundleCount", "completedAt", "connectivityCheckedAt", "credentialCount",
      "preflightCheckedAt", "profileCount", "schemaVersion", "startedAt", "toolCount"
    ].sort());
  });

  it("canonicalizes equivalent Profile and I/O map order into one binding", () => {
    const config = enabledConfig();
    const io = productionIo();
    const reorderedConfig: ExternalMcpConfig = {
      enabled: true,
      profiles: [...config.profiles].reverse().map(profile => ({
        ...profile,
        allowedHosts: [...profile.allowedHosts].reverse(),
        allowedPorts: [...profile.allowedPorts].reverse(),
        allowedTools: [...profile.allowedTools].reverse()
      }))
    };
    const reorderedIo: ExternalMcpProductionIoConfig = {
      credentialCatalogPath: io.credentialCatalogPath,
      secretProvider: {
        providerId: io.secretProvider.providerId,
        keys: Object.fromEntries(Object.entries(io.secretProvider.keys).reverse()),
        secrets: Object.fromEntries(Object.entries(io.secretProvider.secrets).reverse())
      },
      caBundles: Object.fromEntries(Object.entries(io.caBundles).reverse())
    };

    const first = externalMcpReadinessBindingSha256(config, io, bindingOptions());
    const second = externalMcpReadinessBindingSha256(reorderedConfig, reorderedIo, bindingOptions());
    expect(first).toBe(second);
    expect(first).toMatch(/^[a-f0-9]{64}$/);
  });

  it("changes the binding for policy, credential topology, path or effective bound drift", () => {
    const config = enabledConfig();
    const io = productionIo();
    const baseline = externalMcpReadinessBindingSha256(config, io, bindingOptions());
    const variants: Array<[ExternalMcpConfig, ExternalMcpProductionIoConfig, ReturnType<typeof bindingOptions>]> = [
      [{ enabled: true, profiles: config.profiles.map((profile, index) => index === 0
        ? { ...profile, allowedTools: [...profile.allowedTools, "new_tool"] } : profile) }, io, bindingOptions()],
      [config, { ...io, credentialCatalogPath: "/run/dipole/rotated-catalog.json" }, bindingOptions()],
      [config, { ...io, secretProvider: { ...io.secretProvider, secrets: {
        ...io.secretProvider.secrets,
        "SECRET-0123456789ABCDEF": { keyRef: "KEY-0123456789ABCDEF", path: "/run/dipole/rotated-secret.bin" }
      } } }, bindingOptions()],
      [config, io, { ...bindingOptions(), maximumSecretBytes: 2048 }]
    ];
    for (const [variantConfig, variantIo, options] of variants) {
      expect(externalMcpReadinessBindingSha256(variantConfig, variantIo, options)).not.toBe(baseline);
    }
  });

  it("collects exact local and online receipts in one fresh low-sensitivity bundle", async () => {
    const times = clock([
      "2026-08-28T14:00:00.000Z",
      "2026-08-28T14:00:03.000Z"
    ]);
    const preflight = vi.fn(async () => ({
      schemaVersion: "dipole.agent.external-mcp-production-io-preflight.v1" as const,
      enabled: true,
      checkedAt: "2026-08-28T14:00:01.000Z",
      profileCount: 2,
      credentialCount: 2,
      caBundleCount: 2
    }));
    const shadowConnectivityDrill = vi.fn(async () => ({
      schemaVersion: "dipole.agent.external-mcp-shadow-connectivity.v1" as const,
      checkedAt: "2026-08-28T14:00:02.000Z",
      toolCount: 2
    }));
    const collect = createExternalMcpReadinessEvidenceCollector(
      enabledConfig(), productionIo(), { preflight, shadowConnectivityDrill },
      { ...bindingOptions(), now: times, maximumCollectionMs: 10_000 }
    );

    const evidence = await collect({ profileId: "github-prod", tenantId: "dipole" });

    expect(evidence).toEqual({
      schemaVersion: externalMcpReadinessEvidenceSchemaVersion,
      bindingSha256: externalMcpReadinessBindingSha256(enabledConfig(), productionIo(), bindingOptions()),
      startedAt: "2026-08-28T14:00:00.000Z",
      completedAt: "2026-08-28T14:00:03.000Z",
      preflightCheckedAt: "2026-08-28T14:00:01.000Z",
      connectivityCheckedAt: "2026-08-28T14:00:02.000Z",
      profileCount: 2,
      credentialCount: 2,
      caBundleCount: 2,
      toolCount: 2
    });
    expect(preflight).toHaveBeenCalledWith(expect.any(AbortSignal));
    expect(shadowConnectivityDrill).toHaveBeenCalledWith(
      { profileId: "github-prod", tenantId: "dipole" }, expect.any(AbortSignal)
    );
    expect(JSON.stringify(evidence)).not.toMatch(/github|CRED-|SECRET-|KEY-|CA-|\/run\//i);
  });

  it("fails closed on binding, receipt, freshness, cleanup or custom Transport evidence drift", async () => {
    const base = dependencies();
    const cases = [
      {
        config: enabledConfig(), io: productionIo(), dependencies: base,
        options: { ...bindingOptions(), now: clock(["2026-08-28T14:00:00Z", "2026-08-28T14:00:03Z"]), maximumCollectionMs: 2_000 }
      },
      {
        config: enabledConfig(), io: productionIo(), dependencies: {
          ...base, preflight: vi.fn(async () => ({ ...(await base.preflight()), profileCount: 1 }))
        }, options: { ...bindingOptions(), now: clock(["2026-08-28T14:00:00Z", "2026-08-28T14:00:03Z"]) }
      },
      {
        config: enabledConfig(), io: productionIo(), dependencies: {
          ...base, shadowConnectivityDrill: vi.fn(async () => ({ ...(await base.shadowConnectivityDrill()), toolCount: 1 }))
        }, options: { ...bindingOptions(), now: clock(["2026-08-28T14:00:00Z", "2026-08-28T14:00:03Z"]) }
      },
      {
        config: enabledConfig(), io: productionIo(), dependencies: base,
        options: { ...bindingOptions(), trustedTransportBuilder: false, now: clock(["2026-08-28T14:00:00Z"]) }
      },
      {
        config: enabledConfig(), io: productionIo(), dependencies: {
          ...base, preflight: vi.fn(async () => { throw new Error("sensitive readiness path"); })
        }, options: { ...bindingOptions(), now: clock(["2026-08-28T14:00:00Z"]) }
      }
    ];
    for (const candidate of cases) {
      const collect = createExternalMcpReadinessEvidenceCollector(
        candidate.config, candidate.io, candidate.dependencies, candidate.options
      );
      const error = await collect({ profileId: "github-prod", tenantId: "dipole" }).catch((caught: unknown) => caught);
      expect(String(error)).toBe("Error: External MCP readiness evidence failed");
      expect(String(error)).not.toMatch(/sensitive/);
    }
  });

  it("propagates cancellation before collection and between preflight and connectivity", async () => {
    const before = new AbortController();
    before.abort(new Error("cancelled before evidence"));
    const untouched = dependencies();
    const collect = createExternalMcpReadinessEvidenceCollector(
      enabledConfig(), productionIo(), untouched, { ...bindingOptions(), now: () => new Date() }
    );
    await expect(collect({ profileId: "github-prod", tenantId: "dipole" }, before.signal))
      .rejects.toThrow(/cancelled before evidence/i);
    expect(untouched.preflight).not.toHaveBeenCalled();

    const between = new AbortController();
    const boundary = dependencies();
    boundary.preflight.mockImplementationOnce(async () => {
      between.abort(new Error("cancelled after preflight"));
      return preflightReceipt();
    });
    const boundaryCollect = createExternalMcpReadinessEvidenceCollector(
      enabledConfig(), productionIo(), boundary,
      { ...bindingOptions(), now: clock(["2026-08-28T14:00:00Z"]) }
    );
    await expect(boundaryCollect({ profileId: "github-prod", tenantId: "dipole" }, between.signal))
      .rejects.toThrow(/cancelled after preflight/i);
    expect(boundary.shadowConnectivityDrill).not.toHaveBeenCalled();
  });
});

function enabledConfig(): Extract<ExternalMcpConfig, { enabled: true }> {
  const profile = {
    profileId: "github-prod", tenantId: "dipole", serverId: "github-mcp",
    endpoint: "https://mcp.github.example/v1", credentialRef: "CRED-0123456789ABCDEF", credentialVersion: 3,
    allowedHosts: ["mcp.github.example", "backup.github.example"], allowedPorts: [443, 8443],
    dnsResolution: "public_only" as const, tlsServerName: "mcp.github.example",
    caBundleRef: "CA-0123456789ABCDEF", allowedTools: ["read_issue", "list_issues"]
  };
  return { enabled: true, profiles: [profile, {
    ...profile, profileId: "calendar-prod", serverId: "calendar-mcp",
    endpoint: "https://mcp.calendar.example/v1", credentialRef: "CRED-FEDCBA9876543210",
    allowedHosts: ["mcp.calendar.example"], tlsServerName: "mcp.calendar.example",
    caBundleRef: "CA-FEDCBA9876543210", allowedTools: ["read_calendar"]
  }] };
}

function productionIo(): ExternalMcpProductionIoConfig {
  return {
    credentialCatalogPath: "/run/dipole/catalog.json",
    secretProvider: {
      providerId: "local-aes-gcm",
      keys: {
        "KEY-0123456789ABCDEF": "/run/dipole/key-a.bin",
        "KEY-FEDCBA9876543210": "/run/dipole/key-b.bin"
      },
      secrets: {
        "SECRET-0123456789ABCDEF": { keyRef: "KEY-0123456789ABCDEF", path: "/run/dipole/secret-a.bin" },
        "SECRET-FEDCBA9876543210": { keyRef: "KEY-FEDCBA9876543210", path: "/run/dipole/secret-b.bin" }
      }
    },
    caBundles: {
      "CA-0123456789ABCDEF": "/run/dipole/ca-a.pem",
      "CA-FEDCBA9876543210": "/run/dipole/ca-b.pem"
    }
  };
}

function bindingOptions() {
  return {
    expectedOwnerUid: 1000,
    maximumCatalogBytes: 262_144,
    maximumSecretBytes: 4096,
    maximumCaBundleBytes: 262_144,
    connectTimeoutMs: 5_000,
    trustedTransportBuilder: true
  };
}

function dependencies() {
  return {
    preflight: vi.fn(async () => preflightReceipt()),
    shadowConnectivityDrill: vi.fn(async () => ({
      schemaVersion: "dipole.agent.external-mcp-shadow-connectivity.v1" as const,
      checkedAt: "2026-08-28T14:00:02.000Z",
      toolCount: 2
    }))
  };
}

function preflightReceipt() {
  return {
    schemaVersion: "dipole.agent.external-mcp-production-io-preflight.v1" as const,
    enabled: true,
    checkedAt: "2026-08-28T14:00:01.000Z",
    profileCount: 2,
    credentialCount: 2,
    caBundleCount: 2
  };
}

function clock(values: readonly string[]): () => Date {
  const remaining = [...values];
  return () => new Date(remaining.shift() ?? values[values.length - 1]!);
}
