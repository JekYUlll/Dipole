import { describe, expect, it, vi } from "vitest";

import type { ExternalMcpCredentialBinding, ExternalMcpCredentialCatalog } from "./external-mcp-credential-catalog.js";
import type { ExternalMcpConfig, ExternalMcpProfile } from "./external-mcp-profile.js";
import {
  createExternalMcpProductionIoPreflight,
  externalMcpProductionIoPreflightSchemaVersion
} from "./external-mcp-production-io-preflight.js";

const credential: ExternalMcpCredentialBinding = {
  tenantId: "dipole",
  credentialRef: "CRED-0123456789ABCDEF",
  credentialVersion: 3,
  providerId: "local-aes-gcm",
  providerSecretRef: "SECRET-0123456789ABCDEF"
};
const profile: ExternalMcpProfile = {
  profileId: "github-prod",
  tenantId: "dipole",
  serverId: "github-mcp",
  endpoint: "https://mcp.github.example/v1",
  credentialRef: credential.credentialRef,
  credentialVersion: credential.credentialVersion,
  allowedHosts: ["mcp.github.example"],
  allowedPorts: [443],
  dnsResolution: "public_only",
  tlsServerName: "mcp.github.example",
  caBundleRef: "CA-0123456789ABCDEF",
  allowedTools: ["read_issue"]
};
const enabledConfig: ExternalMcpConfig = {
  enabled: true,
  profiles: [profile, { ...profile, profileId: "github-secondary", serverId: "github-mcp-secondary" }]
};

describe("external MCP production I/O preflight", () => {
  it("returns a low-sensitivity disabled receipt without dependencies", async () => {
    const preflight = createExternalMcpProductionIoPreflight({ enabled: false, profiles: [] }, undefined, () => fixedNow());
    await expect(preflight()).resolves.toEqual({
      schemaVersion: externalMcpProductionIoPreflightSchemaVersion,
      enabled: false,
      checkedAt: "2026-08-28T12:00:00.000Z",
      profileCount: 0,
      credentialCount: 0,
      caBundleCount: 0
    });
  });

  it("resolves every Profile at one time and deduplicates exact Secret and CA reads", async () => {
    const source = Buffer.from("token-0123456789");
    const catalog: ExternalMcpCredentialCatalog = { resolve: vi.fn(async () => credential) };
    const secretProvider = { read: vi.fn(async () => source) };
    const caBundles = { read: vi.fn(async () => Buffer.from("validated-ca")) };
    const preflight = createExternalMcpProductionIoPreflight(enabledConfig, {
      catalog, secretProvider, caBundles, maximumSecretBytes: 32
    }, () => fixedNow());

    const receipt = await preflight();
    expect(receipt).toEqual({
      schemaVersion: externalMcpProductionIoPreflightSchemaVersion,
      enabled: true,
      checkedAt: "2026-08-28T12:00:00.000Z",
      profileCount: 2,
      credentialCount: 1,
      caBundleCount: 1
    });
    expect(catalog.resolve).toHaveBeenCalledTimes(2);
    expect(catalog.resolve).toHaveBeenNthCalledWith(1, expect.objectContaining({ now: fixedNow() }));
    expect(secretProvider.read).toHaveBeenCalledOnce();
    expect(caBundles.read).toHaveBeenCalledOnce();
    expect(source.every(byte => byte === 0)).toBe(true);
    expect(JSON.stringify(receipt)).not.toMatch(/github|CRED-|SECRET-|CA-|token/i);
  });

  it("fails closed with a fixed error for revocation, invalid Bearer bytes or CA failure", async () => {
    const cases = [
      {
        catalog: { resolve: vi.fn(async () => { throw new Error("revoked SECRET-sensitive"); }) },
        secretProvider: { read: vi.fn() }, caBundles: { read: vi.fn() }
      },
      {
        catalog: { resolve: vi.fn(async () => credential) },
        secretProvider: { read: vi.fn(async () => Buffer.from("invalid token\n")) },
        caBundles: { read: vi.fn() }
      },
      {
        catalog: { resolve: vi.fn(async () => credential) },
        secretProvider: { read: vi.fn(async () => Buffer.from("valid-token-1234")) },
        caBundles: { read: vi.fn(async () => { throw new Error("bad CA sensitive path"); }) }
      }
    ];
    for (const dependencies of cases) {
      const preflight = createExternalMcpProductionIoPreflight({ enabled: true, profiles: [profile] }, {
        ...dependencies, maximumSecretBytes: 32
      }, () => fixedNow());
      const error = await preflight().catch((caught: unknown) => caught);
      expect(error).toBeInstanceOf(Error);
      expect(String(error)).toBe("Error: External MCP production I/O preflight failed");
      expect(String(error)).not.toMatch(/SECRET-sensitive|sensitive path/);
    }

    const clockFailure = createExternalMcpProductionIoPreflight(
      { enabled: false, profiles: [] }, undefined,
      () => { throw new Error("sensitive clock source"); }
    );
    await expect(clockFailure()).rejects.toThrow("External MCP production I/O preflight failed");
  });

  it("honors cancellation before access and between Catalog and Secret boundaries", async () => {
    const before = new AbortController();
    before.abort(new Error("cancelled before preflight"));
    const untouched = dependencies();
    const preflight = createExternalMcpProductionIoPreflight({ enabled: true, profiles: [profile] }, untouched, () => fixedNow());
    await expect(preflight(before.signal)).rejects.toThrow(/cancelled before preflight/i);
    expect(untouched.catalog.resolve).not.toHaveBeenCalled();

    const between = new AbortController();
    const boundary = dependencies();
    boundary.catalog.resolve.mockImplementationOnce(async () => {
      between.abort(new Error("cancelled after Catalog"));
      return credential;
    });
    const boundaryPreflight = createExternalMcpProductionIoPreflight(
      { enabled: true, profiles: [profile] }, boundary, () => fixedNow()
    );
    await expect(boundaryPreflight(between.signal)).rejects.toThrow(/cancelled after Catalog/i);
    expect(boundary.secretProvider.read).not.toHaveBeenCalled();
    expect(boundary.caBundles.read).not.toHaveBeenCalled();
  });
});

function dependencies() {
  return {
    catalog: { resolve: vi.fn(async () => credential) },
    secretProvider: { read: vi.fn(async () => Buffer.from("valid-token-1234")) },
    caBundles: { read: vi.fn(async () => Buffer.from("validated-ca")) },
    maximumSecretBytes: 32
  };
}

function fixedNow(): Date {
  return new Date("2026-08-28T12:00:00Z");
}
