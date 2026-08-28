import type { AuthProvider, StreamableHTTPClientTransportOptions, Transport } from "@modelcontextprotocol/client";
import { createCipheriv, randomBytes } from "node:crypto";
import { chmod, mkdtemp, readFile, rename, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import type { ExternalMcpCredentialBinding } from "../mcp/external-mcp-credential-catalog.js";
import { loadExternalMcpConfig } from "../mcp/external-mcp-profile.js";
import {
  createExternalMcpProductionIoRegistry,
  type ExternalMcpProductionIoConfig
} from "../mcp/external-mcp-production-io.js";
import { externalMcpEncryptedSecretEnvelopeMagic } from "../mcp/node-external-mcp-encrypted-secret-provider.js";
import { createExternalMcpCredentialLifecycleEvidence } from "./external-mcp-credential-lifecycle-evidence.js";

const enabled = process.env.DIPOLE_AGENT_CREDENTIAL_LIFECYCLE_DRILL === "true";
const integration = enabled ? describe : describe.skip;
const credentialRef = "CRED-0123456789ABCDEF";
const providerId = "local-aes-gcm";
const caRef = "CA-0123456789ABCDEF";

integration("external MCP encrypted credential lifecycle drill", () => {
  it("rotates by version and rejects revoked bindings before Transport construction", async () => {
    const directory = await mkdtemp(join(tmpdir(), "dipole-mcp-credential-drill-"));
    const files = lifecycleFiles(directory);
    const keys = [randomBytes(32), randomBytes(32)] as const;
    const observation = { opened: 0, closed: 0, captures: [] as StreamableHTTPClientTransportOptions[] };
    try {
      await Promise.all([
        writeFile(files.keyV3, keys[0], { mode: 0o600 }),
        writeFile(files.keyV4, keys[1], { mode: 0o600 })
      ]);
      await writeEnvelope(files.secretV3, "drill-token-v3", keys[0], binding(3), files.keyRefV3);
      await writeEnvelope(files.secretV4, "drill-token-v4", keys[1], binding(4), files.keyRefV4);

      await writeCatalog(files, [activeBinding(3)]);
      const runtimeV3 = registry(3, files, observation);
      await expect(connectReadClose(runtimeV3, observation)).resolves.toBe("drill-token-v3");

      await writeCatalog(files, [activeBinding(3), activeBinding(4)]);
      const runtimeV4 = registry(4, files, observation);
      await expect(connectReadClose(runtimeV4, observation)).resolves.toBe("drill-token-v4");

      await writeCatalog(files, [revokedBinding(3), activeBinding(4)]);
      const openedBeforeOldDenial = observation.opened;
      await expect(runtimeV3.connect("github-prod", "dipole")).rejects.toThrow(/revoked/i);
      expect(observation.opened).toBe(openedBeforeOldDenial);

      const restartedV4 = registry(4, files, observation);
      await expect(connectReadClose(restartedV4, observation)).resolves.toBe("drill-token-v4");

      await writeCatalog(files, [revokedBinding(3), revokedBinding(4)]);
      const openedBeforeActiveDenial = observation.opened;
      await expect(restartedV4.connect("github-prod", "dipole")).rejects.toThrow(/revoked/i);
      expect(observation.opened).toBe(openedBeforeActiveDenial);
      expect(observation).toMatchObject({ opened: 3, closed: 3 });

      const evidence = createExternalMcpCredentialLifecycleEvidence({
        initial_credential_verified: true,
        rotated_credential_verified: true,
        old_version_revoked_before_transport: true,
        restart_recovered: true,
        active_version_revoked_before_transport: true,
        transport_open_count: 3,
        transport_close_count: 3,
        inflight_revocation_authority: false
      });
      const serialized = `${JSON.stringify(evidence, null, 2)}\n`;
      expect(serialized).not.toMatch(/drill-token|CRED-|SECRET-|KEY-|github-prod|tenant_id|\/tmp\//i);
      await writeFile(requiredEnv("DIPOLE_AGENT_CREDENTIAL_LIFECYCLE_EVIDENCE"), serialized, { mode: 0o600 });
    } finally {
      keys[0].fill(0);
      keys[1].fill(0);
      await rm(directory, { recursive: true, force: true });
    }
  });
});

function registry(
  version: 3 | 4,
  files: ReturnType<typeof lifecycleFiles>,
  observation: { opened: number; closed: number; captures: StreamableHTTPClientTransportOptions[] }
) {
  return createExternalMcpProductionIoRegistry(enabledProfile(version), productionIo(files), {
    now: () => new Date("2026-08-28T12:00:00.000Z"),
    transportBuilder: {
      create: (_url, options) => {
        observation.opened += 1;
        observation.captures.push(options);
        return {
          close: async () => { observation.closed += 1; },
          send: async () => undefined,
          start: async () => undefined
        } as unknown as Transport;
      }
    }
  });
}

async function connectReadClose(
  runtime: ReturnType<typeof registry>,
  observation: { captures: StreamableHTTPClientTransportOptions[] }
): Promise<string | undefined> {
  const transport = await runtime.connect("github-prod", "dipole");
  try {
    return await (observation.captures.at(-1)!.authProvider as AuthProvider).token();
  } finally {
    await transport.close();
  }
}

function enabledProfile(version: 3 | 4) {
  return loadExternalMcpConfig({
    DIPOLE_AGENT_EXTERNAL_MCP_ENABLED: "true",
    DIPOLE_AGENT_EXTERNAL_MCP_PROFILES: JSON.stringify([{
      schema_version: "dipole.agent.external-mcp-profile.v1",
      profile_id: "github-prod",
      tenant_id: "dipole",
      server_id: "github-mcp",
      endpoint: "https://mcp.github.example/v1",
      credential: { ref: credentialRef, version },
      network_policy: {
        allowed_hosts: ["mcp.github.example"], allowed_ports: [443], dns_resolution: "public_only",
        tls_server_name: "mcp.github.example", ca_bundle_ref: caRef
      },
      allowed_tools: ["read_issue"]
    }])
  });
}

function productionIo(files: ReturnType<typeof lifecycleFiles>): ExternalMcpProductionIoConfig {
  return {
    credentialCatalogPath: files.catalog,
    secretProvider: {
      providerId,
      keys: { [files.keyRefV3]: files.keyV3, [files.keyRefV4]: files.keyV4 },
      secrets: {
        [files.secretRefV3]: { keyRef: files.keyRefV3, path: files.secretV3 },
        [files.secretRefV4]: { keyRef: files.keyRefV4, path: files.secretV4 }
      }
    },
    caBundles: { [caRef]: files.ca }
  };
}

function lifecycleFiles(directory: string) {
  return {
    catalog: join(directory, "catalog.json"), ca: join(directory, "unused-ca.pem"),
    keyRefV3: "KEY-0123456789ABCDEF", keyRefV4: "KEY-FEDCBA9876543210",
    secretRefV3: "SECRET-0123456789ABCDEF", secretRefV4: "SECRET-FEDCBA9876543210",
    keyV3: join(directory, "key-v3.bin"), keyV4: join(directory, "key-v4.bin"),
    secretV3: join(directory, "secret-v3.bin"), secretV4: join(directory, "secret-v4.bin")
  } as const;
}

function binding(version: 3 | 4): ExternalMcpCredentialBinding {
  return {
    tenantId: "dipole", credentialRef, credentialVersion: version, providerId,
    providerSecretRef: version === 3 ? "SECRET-0123456789ABCDEF" : "SECRET-FEDCBA9876543210"
  };
}

function activeBinding(version: 3 | 4) {
  return catalogBinding(version, "active");
}

function revokedBinding(version: 3 | 4) {
  return catalogBinding(version, "revoked");
}

function catalogBinding(version: 3 | 4, status: "active" | "revoked") {
  const credential = binding(version);
  return {
    tenant_id: credential.tenantId,
    credential_ref: credential.credentialRef,
    version: credential.credentialVersion,
    provider_id: credential.providerId,
    provider_secret_ref: credential.providerSecretRef,
    status,
    valid_from: "2026-08-28T00:00:00Z",
    expires_at: null,
    revoked_at: status === "revoked" ? "2026-08-28T11:00:00Z" : null
  };
}

async function writeCatalog(
  files: ReturnType<typeof lifecycleFiles>,
  bindings: readonly ReturnType<typeof catalogBinding>[]
): Promise<void> {
  const replacement = `${files.catalog}.next`;
  await writeFile(replacement, JSON.stringify({
    schema_version: "dipole.agent.external-mcp-credential-catalog.v1", bindings
  }), { mode: 0o600 });
  await chmod(replacement, 0o600);
  await rename(replacement, files.catalog);
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
    "dipole.agent.external-mcp-secret.v1", credential.tenantId, credential.credentialRef,
    String(credential.credentialVersion), credential.providerId, credential.providerSecretRef, keyRef
  ].join("\0"), "utf8"));
  const ciphertext = Buffer.concat([cipher.update(token, "utf8"), cipher.final()]);
  await writeFile(path, Buffer.concat([
    externalMcpEncryptedSecretEnvelopeMagic, nonce, ciphertext, cipher.getAuthTag()
  ]), { mode: 0o600 });
  await chmod(path, 0o600);
}

function requiredEnv(name: string): string {
  const value = process.env[name]?.trim();
  if (!value) throw new Error(`${name} is required`);
  return value;
}
