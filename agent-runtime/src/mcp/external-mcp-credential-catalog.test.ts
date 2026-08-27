import { readFile } from "node:fs/promises";
import { describe, expect, it } from "vitest";

import {
  ReloadingExternalMcpCredentialCatalog,
  externalMcpCredentialCatalogSchemaVersion
} from "./external-mcp-credential-catalog.js";

const activeBinding = {
  tenant_id: "dipole",
  credential_ref: "CRED-0123456789ABCDEF",
  version: 3,
  provider_id: "vault-prod",
  provider_secret_ref: "SECRET-0123456789ABCDEF",
  status: "active",
  valid_from: "2026-08-27T00:00:00Z",
  expires_at: "2026-09-27T00:00:00Z",
  revoked_at: null
};

function manifest(bindings: readonly unknown[] = [activeBinding]): unknown {
  return {
    schema_version: "dipole.agent.external-mcp-credential-catalog.v1",
    bindings
  };
}

describe("external MCP credential lifecycle catalog", () => {
  it("keeps the language-neutral contract aligned with the runtime version", async () => {
    const path = new URL("../../../contracts/agent-external-mcp/v1/credential-catalog.schema.json", import.meta.url);
    const schema = JSON.parse(await readFile(path, "utf8")) as {
      $id: string;
      "x-dipole-version": string;
      properties: { schema_version: { const: string } };
    };
    expect(schema.$id).toMatch(/agent-external-mcp\/v1\/credential-catalog\.schema\.json$/);
    expect(schema["x-dipole-version"]).toBe(externalMcpCredentialCatalogSchemaVersion);
    expect(schema.properties.schema_version.const).toBe(externalMcpCredentialCatalogSchemaVersion);
  });

  it("resolves only an exact active tenant/ref/version to opaque provider references", async () => {
    const catalog = new ReloadingExternalMcpCredentialCatalog(async () => manifest());
    await expect(catalog.resolve({
      tenantId: "dipole",
      credentialRef: "CRED-0123456789ABCDEF",
      credentialVersion: 3,
      now: new Date("2026-08-27T12:00:00Z")
    })).resolves.toEqual({
      tenantId: "dipole",
      credentialRef: "CRED-0123456789ABCDEF",
      credentialVersion: 3,
      providerId: "vault-prod",
      providerSecretRef: "SECRET-0123456789ABCDEF"
    });
  });

  it("accepts RFC 3339 timestamps with an explicit offset", async () => {
    const catalog = new ReloadingExternalMcpCredentialCatalog(async () => manifest([{
      ...activeBinding,
      valid_from: "2026-08-27T08:00:00+08:00",
      expires_at: "2026-09-27T08:00:00+08:00"
    }]));
    await expect(catalog.resolve({
      tenantId: "dipole",
      credentialRef: activeBinding.credential_ref,
      credentialVersion: 3,
      now: new Date("2026-08-27T12:00:00Z")
    })).resolves.toMatchObject({ credentialVersion: 3 });
  });

  it.each([
    ["different tenant", { tenantId: "other", credentialRef: activeBinding.credential_ref, credentialVersion: 3, now: new Date("2026-08-27T12:00:00Z") }],
    ["different version", { tenantId: "dipole", credentialRef: activeBinding.credential_ref, credentialVersion: 4, now: new Date("2026-08-27T12:00:00Z") }],
    ["before activation", { tenantId: "dipole", credentialRef: activeBinding.credential_ref, credentialVersion: 3, now: new Date("2026-08-26T23:59:59Z") }],
    ["at expiry", { tenantId: "dipole", credentialRef: activeBinding.credential_ref, credentialVersion: 3, now: new Date("2026-09-27T00:00:00Z") }]
  ])("fails closed for %s", async (_name, request) => {
    const catalog = new ReloadingExternalMcpCredentialCatalog(async () => manifest());
    await expect(catalog.resolve(request)).rejects.toThrow();
  });

  it("observes rotation and revocation from a reloaded manifest", async () => {
    let current = manifest();
    const catalog = new ReloadingExternalMcpCredentialCatalog(async () => current);
    const request = {
      tenantId: "dipole",
      credentialRef: activeBinding.credential_ref,
      credentialVersion: 3,
      now: new Date("2026-08-27T12:00:00Z")
    };
    await expect(catalog.resolve(request)).resolves.toMatchObject({ credentialVersion: 3 });

    current = manifest([{
      ...activeBinding,
      status: "revoked",
      revoked_at: "2026-08-27T12:01:00Z"
    }, {
      ...activeBinding,
      version: 4,
      provider_secret_ref: "SECRET-FEDCBA9876543210",
      valid_from: "2026-08-27T12:01:00Z"
    }]);
    await expect(catalog.resolve({ ...request, now: new Date("2026-08-27T12:02:00Z") })).rejects.toThrow(/revoked/i);
    await expect(catalog.resolve({ ...request, credentialVersion: 4, now: new Date("2026-08-27T12:02:00Z") })).resolves.toMatchObject({
      credentialVersion: 4,
      providerSecretRef: "SECRET-FEDCBA9876543210"
    });
  });

  it.each([
    ["duplicate binding", manifest([activeBinding, activeBinding])],
    ["active binding with revocation time", manifest([{ ...activeBinding, revoked_at: "2026-08-27T12:00:00Z" }])],
    ["revoked binding without revocation time", manifest([{ ...activeBinding, status: "revoked" }])],
    ["invalid time window", manifest([{ ...activeBinding, expires_at: activeBinding.valid_from }])],
    ["raw secret field", manifest([{ ...activeBinding, secret_value: "must-not-enter-catalog" }])],
    ["malformed provider ref", manifest([{ ...activeBinding, provider_secret_ref: "plain-token" }])]
  ])("rejects %s", async (_name, value) => {
    const catalog = new ReloadingExternalMcpCredentialCatalog(async () => value);
    await expect(catalog.resolve({
      tenantId: "dipole",
      credentialRef: activeBinding.credential_ref,
      credentialVersion: 3,
      now: new Date("2026-08-27T12:00:00Z")
    })).rejects.toThrow();
  });

  it("does not cache a failed or stale loader result", async () => {
    let attempt = 0;
    const catalog = new ReloadingExternalMcpCredentialCatalog(async () => {
      attempt += 1;
      if (attempt === 1) throw new Error("catalog unavailable");
      return manifest();
    });
    const request = {
      tenantId: "dipole",
      credentialRef: activeBinding.credential_ref,
      credentialVersion: 3,
      now: new Date("2026-08-27T12:00:00Z")
    };
    await expect(catalog.resolve(request)).rejects.toThrow(/unavailable/);
    await expect(catalog.resolve(request)).resolves.toMatchObject({ providerId: "vault-prod" });
  });
});
