import { chmod, link, mkdir, mkdtemp, readFile, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import {
  externalMcpProductionIoManifestSchemaVersion,
  loadExternalMcpProductionIoManifest
} from "./external-mcp-production-io-manifest.js";
import { createExternalMcpProductionIoRegistry } from "./external-mcp-production-io.js";
import { loadExternalMcpConfig } from "./external-mcp-profile.js";

const keyRef = "KEY-0123456789ABCDEF";
const secretRef = "SECRET-0123456789ABCDEF";
const caRef = "CA-0123456789ABCDEF";
let directory = "";
let manifestPath = "";

beforeAll(async () => {
  directory = await mkdtemp(join(tmpdir(), "dipole-mcp-io-manifest-"));
  manifestPath = join(directory, "production-io.json");
});

afterAll(async () => {
  await rm(directory, { recursive: true, force: true });
});

describe("external MCP production I/O manifest", () => {
  it("keeps the language-neutral contract aligned with the Runtime version", async () => {
    const path = new URL("../../../../contracts/agent-external-mcp/v1/production-io-manifest.schema.json", import.meta.url);
    const schema = JSON.parse(await readFile(path, "utf8")) as {
      $id: string;
      "x-dipole-version": string;
      properties: { schema_version: { const: string } };
    };
    expect(schema.$id).toMatch(/agent-external-mcp\/v1\/production-io-manifest\.schema\.json$/);
    expect(schema["x-dipole-version"]).toBe(externalMcpProductionIoManifestSchemaVersion);
    expect(schema.properties.schema_version.const).toBe(externalMcpProductionIoManifestSchemaVersion);
  });

  it("does not inspect a residual manifest path while external MCP is disabled", async () => {
    const env = {} as NodeJS.ProcessEnv;
    Object.defineProperty(env, "DIPOLE_AGENT_EXTERNAL_MCP_IO_MANIFEST", {
      get: () => { throw new Error("disabled loader touched residual manifest config"); }
    });
    await expect(loadExternalMcpProductionIoManifest(loadExternalMcpConfig({}), env)).resolves.toBeUndefined();
  });

  it("loads a strict credential-free manifest into production composition input", async () => {
    await writeManifest(validManifest());
    const loaded = await loadExternalMcpProductionIoManifest(enabledProfiles(), {
      DIPOLE_AGENT_EXTERNAL_MCP_IO_MANIFEST: manifestPath
    });
    expect(loaded).toEqual({
      io: {
        credentialCatalogPath: join(directory, "catalog.json"),
        secretProvider: {
          providerId: "local-aes-gcm",
          keys: { [keyRef]: join(directory, "key.bin") },
          secrets: { [secretRef]: { keyRef, path: join(directory, "secret.bin") } }
        },
        caBundles: { [caRef]: join(directory, "ca.pem") }
      },
      options: expect.objectContaining({
        maximumCatalogBytes: 262144,
        maximumSecretBytes: 4096,
        maximumCaBundleBytes: 262144,
        connectTimeoutMs: 5000
      })
    });
    const registry = createExternalMcpProductionIoRegistry(enabledProfiles(), loaded!.io, loaded!.options);
    expect(registry.describe("github-prod", "dipole")).toMatchObject({ serverId: "github-mcp" });
  });

  it("reloads the manifest without retaining a previous successful snapshot", async () => {
    await writeManifest(validManifest());
    const first = await load();
    await writeManifest({
      ...validManifest(),
      ca_bundles: {
        ...validManifest().ca_bundles,
        entries: [{ ca_bundle_ref: caRef, path: join(directory, "rotated-ca.pem") }]
      }
    });
    const second = await load();
    expect(first!.io.caBundles[caRef]).toBe(join(directory, "ca.pem"));
    expect(second!.io.caBundles[caRef]).toBe(join(directory, "rotated-ca.pem"));

    await writeFile(manifestPath, "not-json secret-token-value", { mode: 0o600 });
    const error = await load().catch((caught: unknown) => caught);
    expect(error).toBeInstanceOf(Error);
    expect(String(error)).not.toContain("secret-token-value");
  });

  it.each([
    ["unknown key", () => ({
      ...validManifest(),
      encrypted_secret_provider: {
        ...validManifest().encrypted_secret_provider,
        secrets: [{ secret_ref: secretRef, key_ref: "KEY-FEDCBA9876543210", path: join(directory, "secret.bin") }]
      }
    })],
    ["duplicate reference", () => ({
      ...validManifest(),
      ca_bundles: {
        ...validManifest().ca_bundles,
        entries: [
          { ca_bundle_ref: caRef, path: join(directory, "ca.pem") },
          { ca_bundle_ref: caRef, path: join(directory, "ca-two.pem") }
        ]
      }
    })],
    ["aliased path", () => ({
      ...validManifest(),
      ca_bundles: {
        ...validManifest().ca_bundles,
        entries: [{ ca_bundle_ref: caRef, path: join(directory, "key.bin") }]
      }
    })],
    ["relative path", () => ({ ...validManifest(), credential_catalog: { path: "catalog.json", maximum_bytes: 262144 } })],
    ["raw secret", () => ({ ...validManifest(), token: "forbidden-secret-value" })]
  ])("rejects %s without exposing manifest values", async (_name, build) => {
    await writeManifest(build());
    const error = await load().catch((caught: unknown) => caught);
    expect(error).toBeInstanceOf(Error);
    expect(String(error)).not.toContain("forbidden-secret-value");
    expect(String(error)).not.toContain(manifestPath);
  });

  it("requires a secure single-link manifest file owned by the Runtime or root", async () => {
    await writeManifest(validManifest());
    const symlinkPath = join(directory, "manifest-link.json");
    await symlink(manifestPath, symlinkPath);
    await expect(load(symlinkPath)).rejects.toThrow(/^External MCP production I\/O manifest is unavailable$/);
    await rm(symlinkPath);

    const hardLinkPath = join(directory, "manifest-hard-link.json");
    await link(manifestPath, hardLinkPath);
    await expect(load()).rejects.toThrow(/^External MCP production I\/O manifest is unavailable$/);
    await rm(hardLinkPath);

    await chmod(manifestPath, 0o666);
    await expect(load()).rejects.toThrow(/^External MCP production I\/O manifest is unavailable$/);
    await chmod(manifestPath, 0o640);
    await expect(load()).rejects.toThrow(/^External MCP production I\/O manifest is unavailable$/);
    await chmod(manifestPath, 0o200);
    await expect(load()).rejects.toThrow(/^External MCP production I\/O manifest is unavailable$/);
    await chmod(manifestPath, 0o600);

    const writableParent = join(directory, "writable-parent");
    await mkdir(writableParent, { mode: 0o700 });
    const writableManifest = join(writableParent, "manifest.json");
    await writeFile(writableManifest, JSON.stringify(validManifest()), { mode: 0o600 });
    await chmod(writableParent, 0o777);
    await expect(load(writableManifest)).rejects.toThrow(/^External MCP production I\/O manifest is unavailable$/);
    await chmod(writableParent, 0o700);

    const canonicalParent = join(directory, "canonical-parent");
    const parentLink = join(directory, "parent-link");
    await mkdir(canonicalParent, { mode: 0o700 });
    await writeFile(join(canonicalParent, "manifest.json"), JSON.stringify(validManifest()), { mode: 0o600 });
    await symlink(canonicalParent, parentLink);
    await expect(load(join(parentLink, "manifest.json"))).rejects.toThrow(/^External MCP production I\/O manifest is unavailable$/);
    await rm(parentLink);
  });

  it("requires an enabled manifest path and honors pre-cancellation", async () => {
    await expect(loadExternalMcpProductionIoManifest(enabledProfiles(), {}))
      .rejects.toThrow(/^External MCP production I\/O manifest is unavailable$/);
    const controller = new AbortController();
    controller.abort(new Error("cancelled before manifest read"));
    await expect(loadExternalMcpProductionIoManifest(enabledProfiles(), {
      DIPOLE_AGENT_EXTERNAL_MCP_IO_MANIFEST: manifestPath
    }, { signal: controller.signal })).rejects.toThrow(/cancelled before manifest read/i);
  });
});

function enabledProfiles() {
  return loadExternalMcpConfig({
    DIPOLE_AGENT_EXTERNAL_MCP_ENABLED: "true",
    DIPOLE_AGENT_EXTERNAL_MCP_PROFILES: JSON.stringify([{
      schema_version: "dipole.agent.external-mcp-profile.v1",
      profile_id: "github-prod",
      tenant_id: "dipole",
      server_id: "github-mcp",
      endpoint: "https://mcp.github.example/v1",
      credential: { ref: "CRED-0123456789ABCDEF", version: 3 },
      network_policy: {
        allowed_hosts: ["mcp.github.example"],
        allowed_ports: [443],
        dns_resolution: "public_only",
        tls_server_name: "mcp.github.example",
        ca_bundle_ref: caRef
      },
      allowed_tools: ["read_issue"]
    }])
  });
}

function validManifest() {
  return {
    schema_version: "dipole.agent.external-mcp-production-io.v1",
    credential_catalog: { path: join(directory, "catalog.json"), maximum_bytes: 262144 },
    encrypted_secret_provider: {
      provider_id: "local-aes-gcm",
      maximum_secret_bytes: 4096,
      keys: [{ key_ref: keyRef, path: join(directory, "key.bin") }],
      secrets: [{ secret_ref: secretRef, key_ref: keyRef, path: join(directory, "secret.bin") }]
    },
    ca_bundles: {
      maximum_bytes: 262144,
      entries: [{ ca_bundle_ref: caRef, path: join(directory, "ca.pem") }]
    },
    tls: { connect_timeout_ms: 5000 }
  };
}

async function writeManifest(value: unknown): Promise<void> {
  await writeFile(manifestPath, JSON.stringify(value), { mode: 0o600 });
  await chmod(manifestPath, 0o600);
}

function load(path = manifestPath) {
  return loadExternalMcpProductionIoManifest(enabledProfiles(), {
    DIPOLE_AGENT_EXTERNAL_MCP_IO_MANIFEST: path
  });
}
