import { z } from "zod";
import { constants } from "node:fs";
import { lstat, open, realpath } from "node:fs/promises";
import { dirname, isAbsolute, normalize } from "node:path";

export const externalMcpCredentialCatalogSchemaVersion = "dipole.agent.external-mcp-credential-catalog.v1" as const;

const identifierSchema = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$/);
const credentialRefSchema = z.string().regex(/^CRED-[A-Z0-9]{16,64}$/);
const providerSecretRefSchema = z.string().regex(/^SECRET-[A-Z0-9]{16,64}$/);
const timestampSchema = z.iso.datetime({ offset: true });

const rawBindingSchema = z.object({
  tenant_id: identifierSchema,
  credential_ref: credentialRefSchema,
  version: z.number().int().positive(),
  provider_id: identifierSchema,
  provider_secret_ref: providerSecretRefSchema,
  status: z.enum(["active", "revoked"]),
  valid_from: timestampSchema,
  expires_at: timestampSchema.nullable(),
  revoked_at: timestampSchema.nullable()
}).strict().superRefine((binding, refinement) => {
  const validFrom = Date.parse(binding.valid_from);
  const expiresAt = binding.expires_at === null ? undefined : Date.parse(binding.expires_at);
  const revokedAt = binding.revoked_at === null ? undefined : Date.parse(binding.revoked_at);
  if (expiresAt !== undefined && expiresAt <= validFrom) {
    refinement.addIssue({ code: "custom", message: "Credential expiry must follow activation", path: ["expires_at"] });
  }
  if (binding.status === "active" && revokedAt !== undefined) {
    refinement.addIssue({ code: "custom", message: "Active credential cannot have a revocation time", path: ["revoked_at"] });
  }
  if (binding.status === "revoked" && revokedAt === undefined) {
    refinement.addIssue({ code: "custom", message: "Revoked credential requires a revocation time", path: ["revoked_at"] });
  }
  if (revokedAt !== undefined && revokedAt < validFrom) {
    refinement.addIssue({ code: "custom", message: "Credential revocation cannot precede activation", path: ["revoked_at"] });
  }
});

const rawCatalogSchema = z.object({
  schema_version: z.literal(externalMcpCredentialCatalogSchemaVersion),
  bindings: z.array(rawBindingSchema).min(1).max(1024)
}).strict().superRefine((catalog, refinement) => {
  const keys = catalog.bindings.map(binding => `${binding.tenant_id}\0${binding.credential_ref}\0${binding.version}`);
  if (new Set(keys).size !== keys.length) {
    refinement.addIssue({ code: "custom", message: "Credential tenant/ref/version bindings must be unique", path: ["bindings"] });
  }
});

export interface ExternalMcpCredentialResolveRequest {
  readonly tenantId: string;
  readonly credentialRef: string;
  readonly credentialVersion: number;
  readonly now: Date;
}

export interface ExternalMcpCredentialBinding {
  readonly tenantId: string;
  readonly credentialRef: string;
  readonly credentialVersion: number;
  readonly providerId: string;
  readonly providerSecretRef: string;
}

export interface ExternalMcpCredentialCatalog {
  resolve(request: ExternalMcpCredentialResolveRequest): Promise<ExternalMcpCredentialBinding>;
}

export interface ExternalMcpCredentialCatalogFileOptions {
  readonly expectedOwnerUid?: number;
  readonly maximumBytes?: number;
}

export function createFileExternalMcpCredentialCatalog(
  path: string,
  options: ExternalMcpCredentialCatalogFileOptions = {}
): ExternalMcpCredentialCatalog {
  if (!isAbsolute(path) || normalize(path) !== path) {
    throw new Error("External MCP credential Catalog path must be absolute and normalized");
  }
  const expectedOwnerUid = options.expectedOwnerUid ?? process.getuid?.();
  if (expectedOwnerUid === undefined || !Number.isSafeInteger(expectedOwnerUid) || expectedOwnerUid < 0) {
    throw new Error("External MCP credential Catalog requires a valid expected owner UID");
  }
  const maximumBytes = options.maximumBytes ?? 256 * 1024;
  if (!Number.isSafeInteger(maximumBytes) || maximumBytes < 32 || maximumBytes > 1024 * 1024) {
    throw new Error("External MCP credential Catalog maximum size must be between 32 bytes and 1 MiB");
  }
  return new ReloadingExternalMcpCredentialCatalog(() => loadCatalogFile(path, expectedOwnerUid, maximumBytes));
}

export class ReloadingExternalMcpCredentialCatalog implements ExternalMcpCredentialCatalog {
  readonly #load: () => Promise<unknown>;

  constructor(load: () => Promise<unknown>) {
    this.#load = load;
  }

  async resolve(request: ExternalMcpCredentialResolveRequest): Promise<ExternalMcpCredentialBinding> {
    if (!Number.isFinite(request.now.getTime())) throw new Error("Credential resolution time is invalid");
    const catalog = rawCatalogSchema.parse(await this.#load());
    const binding = catalog.bindings.find(candidate =>
      candidate.tenant_id === request.tenantId
      && candidate.credential_ref === request.credentialRef
      && candidate.version === request.credentialVersion
    );
    if (binding === undefined) throw new Error("External MCP credential binding is not configured");
    if (binding.status === "revoked") throw new Error("External MCP credential binding is revoked");
    const now = request.now.getTime();
    if (now < Date.parse(binding.valid_from)) throw new Error("External MCP credential binding is not active yet");
    if (binding.expires_at !== null && now >= Date.parse(binding.expires_at)) {
      throw new Error("External MCP credential binding is expired");
    }
    return {
      tenantId: binding.tenant_id,
      credentialRef: binding.credential_ref,
      credentialVersion: binding.version,
      providerId: binding.provider_id,
      providerSecretRef: binding.provider_secret_ref
    };
  }
}

async function loadCatalogFile(path: string, expectedOwnerUid: number, maximumBytes: number): Promise<unknown> {
  const parent = dirname(path);
  if (await realpath(parent) !== parent) throw new Error("External MCP credential Catalog parent must be canonical without symlinks");
  const parentStats = await lstat(parent);
  if (!parentStats.isDirectory()) throw new Error("External MCP credential Catalog parent must be a directory");
  if (parentStats.uid !== 0 && parentStats.uid !== expectedOwnerUid) throw new Error("External MCP credential Catalog parent has an unexpected owner");
  if ((parentStats.mode & 0o022) !== 0) throw new Error("External MCP credential Catalog parent cannot be group or world writable");

  const handle = await open(path, constants.O_RDONLY | constants.O_NOFOLLOW);
  try {
    const stats = await handle.stat();
    if (!stats.isFile()) throw new Error("External MCP credential Catalog must be a regular file");
    if (stats.nlink !== 1) throw new Error("External MCP credential Catalog must have exactly one hard link");
    if (stats.uid !== 0 && stats.uid !== expectedOwnerUid) throw new Error("External MCP credential Catalog has an unexpected owner");
    if ((stats.mode & 0o022) !== 0) throw new Error("External MCP credential Catalog cannot be group or world writable");
    if (stats.size === 0 || stats.size > maximumBytes) throw new Error("External MCP credential Catalog size is outside the configured bound");

    const buffer = Buffer.allocUnsafe(maximumBytes + 1);
    let total = 0;
    while (total < buffer.length) {
      const result = await handle.read(buffer, total, buffer.length - total, null);
      if (result.bytesRead === 0) break;
      total += result.bytesRead;
    }
    if (total === 0 || total > maximumBytes) throw new Error("External MCP credential Catalog size is outside the configured bound");
    try {
      return JSON.parse(buffer.subarray(0, total).toString("utf8")) as unknown;
    } catch {
      throw new Error("External MCP credential Catalog must contain valid JSON");
    }
  } finally {
    await handle.close();
  }
}
