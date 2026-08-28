import type { AuthProvider, StreamableHTTPClientTransportOptions, Transport } from "@modelcontextprotocol/client";
import { execFile } from "node:child_process";
import { createCipheriv, randomBytes } from "node:crypto";
import { chmod, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { promisify } from "node:util";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";

import type { ExternalMcpCredentialBinding } from "./external-mcp-credential-catalog.js";
import { loadExternalMcpConfig } from "./external-mcp-profile.js";
import {
  createExternalMcpProductionIoRegistry,
  createExternalMcpProductionIoRuntime,
  type ExternalMcpProductionIoConfig
} from "./external-mcp-production-io.js";
import { externalMcpEncryptedSecretEnvelopeMagic } from "./node-external-mcp-encrypted-secret-provider.js";

const execFileAsync = promisify(execFile);
const credentialRef = "CRED-0123456789ABCDEF";
const firstSecretRef = "SECRET-0123456789ABCDEF";
const secondSecretRef = "SECRET-FEDCBA9876543210";
const firstKeyRef = "KEY-0123456789ABCDEF";
const secondKeyRef = "KEY-FEDCBA9876543210";
const caRef = "CA-0123456789ABCDEF";
let directory = "";
let catalogPath = "";
let firstKeyPath = "";
let secondKeyPath = "";
let firstSecretPath = "";
let secondSecretPath = "";
let caPath = "";
let firstKeyOriginal: Buffer;
let firstSecretOriginal: Buffer;
let caOriginal: Buffer;

beforeAll(async () => {
  directory = await mkdtemp(join(tmpdir(), "dipole-mcp-production-io-"));
  catalogPath = join(directory, "catalog.json");
  firstKeyPath = join(directory, "key-v1.bin");
  secondKeyPath = join(directory, "key-v2.bin");
  firstSecretPath = join(directory, "secret-v1.bin");
  secondSecretPath = join(directory, "secret-v2.bin");
  caPath = join(directory, "ca.pem");
  const firstKey = randomBytes(32);
  const secondKey = randomBytes(32);
  await writeFile(firstKeyPath, firstKey, { mode: 0o600 });
  await writeFile(secondKeyPath, secondKey, { mode: 0o600 });
  await writeEnvelope(firstSecretPath, "token-first", firstKey, binding(firstSecretRef), firstKeyRef);
  await writeEnvelope(secondSecretPath, "token-second", secondKey, binding(secondSecretRef), secondKeyRef);
  await generateCertificate(caPath, join(directory, "ca-key.pem"));
  await chmod(caPath, 0o644);
  firstKeyOriginal = await readFile(firstKeyPath);
  firstSecretOriginal = await readFile(firstSecretPath);
  caOriginal = await readFile(caPath);
  firstKey.fill(0);
  secondKey.fill(0);
});

afterAll(async () => {
  await rm(directory, { recursive: true, force: true });
});

describe("external MCP production I/O composition", () => {
  it("requires no I/O configuration while disabled", () => {
    const residual = {} as ExternalMcpProductionIoConfig;
    Object.defineProperty(residual, "credentialCatalogPath", {
      get: () => { throw new Error("disabled startup touched residual I/O configuration"); }
    });
    const registry = createExternalMcpProductionIoRegistry(loadExternalMcpConfig({}), residual);
    expect(() => registry.describe("github-prod", "dipole")).toThrow(/disabled/i);
  });

  it("keeps the disabled Shadow connectivity drill fail-closed without touching residual I/O", async () => {
    const residual = {} as ExternalMcpProductionIoConfig;
    Object.defineProperty(residual, "credentialCatalogPath", {
      get: () => { throw new Error("disabled Shadow drill touched residual I/O configuration"); }
    });
    const runtime = createExternalMcpProductionIoRuntime(loadExternalMcpConfig({}), residual, {
      now: () => new Date("2026-08-28T13:00:00Z")
    });
    await expect(runtime.shadowConnectivityDrill({ profileId: "github-prod", tenantId: "dipole" }))
      .rejects.toThrow("External MCP Shadow connectivity drill failed");
    await expect(runtime.readinessEvidence({ profileId: "github-prod", tenantId: "dipole" }))
      .rejects.toThrow("External MCP readiness evidence failed");
  });

  it("refuses readiness evidence from an injected Transport builder before file or network access", async () => {
    const create = vi.fn();
    const runtime = createExternalMcpProductionIoRuntime(enabledProfiles(), productionIo(), {
      transportBuilder: { create },
      now: () => new Date("2026-08-28T14:00:00Z")
    });

    await expect(runtime.readinessEvidence({ profileId: "github-prod", tenantId: "dipole" }))
      .rejects.toThrow("External MCP readiness evidence failed");
    expect(create).not.toHaveBeenCalled();
  });

  it("validates enabled construction without reading configured files", () => {
    expect(() => createExternalMcpProductionIoRegistry(enabledProfiles())).toThrow(/I\/O configuration/i);
    const missingRoot = join(directory, "files-that-do-not-exist");
    const registry = createExternalMcpProductionIoRegistry(enabledProfiles(), {
      credentialCatalogPath: join(missingRoot, "catalog.json"),
      secretProvider: {
        providerId: "local-aes-gcm",
        keys: { [firstKeyRef]: join(missingRoot, "key.bin") },
        secrets: { [firstSecretRef]: { keyRef: firstKeyRef, path: join(missingRoot, "secret.bin") } }
      },
      caBundles: { [caRef]: join(missingRoot, "ca.pem") }
    });
    expect(registry.describe("github-prod", "dipole")).toMatchObject({ serverId: "github-mcp" });
  });

  it("reloads Catalog per connect and encrypted bytes per token without opening network state", async () => {
    const captures: StreamableHTTPClientTransportOptions[] = [];
    const create = vi.fn((_url: URL, options: StreamableHTTPClientTransportOptions) => {
      captures.push(options);
      return { close: vi.fn(), send: vi.fn(), start: vi.fn() } as unknown as Transport;
    });
    const io = productionIo();
    await writeCatalog(firstSecretRef, "active");
    const registry = createExternalMcpProductionIoRegistry(enabledProfiles(), io, {
      transportBuilder: { create },
      now: () => new Date("2026-08-28T12:00:00Z")
    });

    await registry.connect("github-prod", "dipole");
    await expect((captures[0]!.authProvider as AuthProvider).token()).resolves.toBe("token-first");

    await writeCatalog(secondSecretRef, "active");
    await registry.connect("github-prod", "dipole");
    await expect((captures[1]!.authProvider as AuthProvider).token()).resolves.toBe("token-second");
    expect(create).toHaveBeenCalledTimes(2);
  });

  it("blocks a revoked Catalog binding before constructing a Transport", async () => {
    const create = vi.fn();
    await writeCatalog(firstSecretRef, "revoked");
    const registry = createExternalMcpProductionIoRegistry(enabledProfiles(), productionIo(), {
      transportBuilder: { create },
      now: () => new Date("2026-08-28T12:00:00Z")
    });
    await expect(registry.connect("github-prod", "dipole")).rejects.toThrow(/revoked/i);
    expect(create).not.toHaveBeenCalled();
  });

  it("isolates a Catalog/provider identity mismatch before DNS or TLS access", async () => {
    const captures: StreamableHTTPClientTransportOptions[] = [];
    const create = vi.fn((_url: URL, options: StreamableHTTPClientTransportOptions) => {
      captures.push(options);
      return { close: vi.fn(), send: vi.fn(), start: vi.fn() } as unknown as Transport;
    });
    await writeCatalog(firstSecretRef, "active", "different-provider");
    const registry = createExternalMcpProductionIoRegistry(enabledProfiles(), productionIo(), {
      transportBuilder: { create },
      now: () => new Date("2026-08-28T12:00:00Z")
    });
    await registry.connect("github-prod", "dipole");
    const error = await (captures[0]!.authProvider as AuthProvider).token().catch((caught: unknown) => caught);
    expect(error).toMatchObject({ code: "secret_unavailable" });
    expect(String(error)).not.toContain(firstSecretRef);
    expect(String(error)).not.toContain("different-provider");
  });

  it("preflights real Catalog, encrypted Secret and CA files without constructing network state", async () => {
    const create = vi.fn();
    const now = new Date("2026-08-28T12:00:00Z");
    await restorePreflightFiles();
    const runtime = createExternalMcpProductionIoRuntime(enabledProfiles(), productionIo(), {
      transportBuilder: { create },
      now: () => now
    });

    try {
      await expect(runtime.preflight()).resolves.toEqual({
        schemaVersion: "dipole.agent.external-mcp-production-io-preflight.v1",
        enabled: true,
        checkedAt: now.toISOString(),
        profileCount: 1,
        credentialCount: 1,
        caBundleCount: 1
      });
      expect(create).not.toHaveBeenCalled();

      await writeFile(firstKeyPath, randomBytes(32), { mode: 0o600 });
      await expect(runtime.preflight()).rejects.toThrow("External MCP production I/O preflight failed");
      await restorePreflightFiles();

      const corruptEnvelope = Buffer.from(firstSecretOriginal);
      const tagIndex = corruptEnvelope.length - 1;
      corruptEnvelope[tagIndex] = corruptEnvelope[tagIndex]! ^ 0xff;
      await writeFile(firstSecretPath, corruptEnvelope, { mode: 0o600 });
      await expect(runtime.preflight()).rejects.toThrow("External MCP production I/O preflight failed");
      await restorePreflightFiles();

      await writeFile(caPath, Buffer.alloc(512, "x"), { mode: 0o644 });
      await expect(runtime.preflight()).rejects.toThrow("External MCP production I/O preflight failed");
      await restorePreflightFiles();

      await writeCatalog(firstSecretRef, "revoked");
      await expect(runtime.preflight()).rejects.toThrow("External MCP production I/O preflight failed");
      expect(create).not.toHaveBeenCalled();
    } finally {
      await restorePreflightFiles();
    }
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
      credential: { ref: credentialRef, version: 3 },
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

function productionIo(): ExternalMcpProductionIoConfig {
  return {
    credentialCatalogPath: catalogPath,
    secretProvider: {
      providerId: "local-aes-gcm",
      keys: { [firstKeyRef]: firstKeyPath, [secondKeyRef]: secondKeyPath },
      secrets: {
        [firstSecretRef]: { keyRef: firstKeyRef, path: firstSecretPath },
        [secondSecretRef]: { keyRef: secondKeyRef, path: secondSecretPath }
      }
    },
    caBundles: { [caRef]: caPath }
  };
}

function binding(providerSecretRef: string): ExternalMcpCredentialBinding {
  return {
    tenantId: "dipole",
    credentialRef,
    credentialVersion: 3,
    providerId: "local-aes-gcm",
    providerSecretRef
  };
}

async function writeCatalog(
  providerSecretRef: string,
  status: "active" | "revoked",
  providerId = "local-aes-gcm"
): Promise<void> {
  await writeFile(catalogPath, JSON.stringify({
    schema_version: "dipole.agent.external-mcp-credential-catalog.v1",
    bindings: [{
      tenant_id: "dipole",
      credential_ref: credentialRef,
      version: 3,
      provider_id: providerId,
      provider_secret_ref: providerSecretRef,
      status,
      valid_from: "2026-08-28T00:00:00Z",
      expires_at: null,
      revoked_at: status === "revoked" ? "2026-08-28T11:00:00Z" : null
    }]
  }), { mode: 0o600 });
  await chmod(catalogPath, 0o600);
}

async function writeEnvelope(
  path: string,
  token: string,
  key: Uint8Array,
  credential: ExternalMcpCredentialBinding,
  keyRef: string
): Promise<void> {
  const nonce = randomBytes(12);
  const cipher = createCipheriv("aes-256-gcm", key, nonce, { authTagLength: 16 });
  cipher.setAAD(Buffer.from([
    "dipole.agent.external-mcp-secret.v1",
    credential.tenantId,
    credential.credentialRef,
    String(credential.credentialVersion),
    credential.providerId,
    credential.providerSecretRef,
    keyRef
  ].join("\0"), "utf8"));
  const ciphertext = Buffer.concat([cipher.update(token, "utf8"), cipher.final()]);
  await writeFile(path, Buffer.concat([
    externalMcpEncryptedSecretEnvelopeMagic,
    nonce,
    ciphertext,
    cipher.getAuthTag()
  ]), { mode: 0o600 });
  await chmod(path, 0o600);
}

async function generateCertificate(certPath: string, keyPath: string): Promise<void> {
  await execFileAsync("openssl", [
    "req", "-x509", "-newkey", "rsa:2048", "-sha256", "-nodes",
    "-keyout", keyPath, "-out", certPath, "-days", "1", "-subj", "/CN=mcp.github.example",
    "-addext", "subjectAltName=DNS:mcp.github.example",
    "-addext", "basicConstraints=critical,CA:TRUE"
  ]);
}

async function restorePreflightFiles(): Promise<void> {
  await writeFile(firstKeyPath, firstKeyOriginal, { mode: 0o600 });
  await chmod(firstKeyPath, 0o600);
  await writeFile(firstSecretPath, firstSecretOriginal, { mode: 0o600 });
  await chmod(firstSecretPath, 0o600);
  await writeFile(caPath, caOriginal, { mode: 0o644 });
  await chmod(caPath, 0o644);
  await writeCatalog(firstSecretRef, "active");
}
