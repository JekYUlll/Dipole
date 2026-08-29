import { isUtf8 } from "node:buffer";
import { constants } from "node:fs";
import { lstat, open, realpath } from "node:fs/promises";
import { dirname, isAbsolute, normalize } from "node:path";
import { z } from "zod";

import type { ExternalMcpConfig } from "./external-mcp-profile.js";
import type {
  ExternalMcpProductionIoConfig,
  ExternalMcpProductionIoOptions
} from "./external-mcp-production-io.js";

export const externalMcpProductionIoManifestSchemaVersion = "dipole.agent.external-mcp-production-io.v1" as const;

const identifierSchema = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$/);
const keyRefSchema = z.string().regex(/^KEY-[A-Z0-9]{16,64}$/);
const secretRefSchema = z.string().regex(/^SECRET-[A-Z0-9]{16,64}$/);
const caRefSchema = z.string().regex(/^CA-[A-Z0-9]{16,64}$/);
const absolutePathSchema = z.string().min(1).max(4096).refine(path => isAbsolute(path) && normalize(path) === path);

const rawManifestSchema = z.object({
  schema_version: z.literal(externalMcpProductionIoManifestSchemaVersion),
  credential_catalog: z.object({
    path: absolutePathSchema,
    maximum_bytes: z.number().int().min(32).max(1024 * 1024)
  }).strict(),
  encrypted_secret_provider: z.object({
    provider_id: identifierSchema,
    maximum_secret_bytes: z.number().int().min(16).max(8192),
    keys: z.array(z.object({
      key_ref: keyRefSchema,
      path: absolutePathSchema
    }).strict()).min(1).max(256),
    secrets: z.array(z.object({
      secret_ref: secretRefSchema,
      key_ref: keyRefSchema,
      path: absolutePathSchema
    }).strict()).min(1).max(256)
  }).strict(),
  ca_bundles: z.object({
    maximum_bytes: z.number().int().min(256).max(1024 * 1024),
    entries: z.array(z.object({
      ca_bundle_ref: caRefSchema,
      path: absolutePathSchema
    }).strict()).min(1).max(256)
  }).strict(),
  tls: z.object({
    connect_timeout_ms: z.number().int().min(100).max(60_000)
  }).strict()
}).strict().superRefine((manifest, refinement) => {
  requireUnique(manifest.encrypted_secret_provider.keys.map(entry => entry.key_ref), ["encrypted_secret_provider", "keys"], refinement);
  requireUnique(manifest.encrypted_secret_provider.secrets.map(entry => entry.secret_ref), ["encrypted_secret_provider", "secrets"], refinement);
  requireUnique(manifest.ca_bundles.entries.map(entry => entry.ca_bundle_ref), ["ca_bundles", "entries"], refinement);
  const keys = new Set(manifest.encrypted_secret_provider.keys.map(entry => entry.key_ref));
  for (const [index, secret] of manifest.encrypted_secret_provider.secrets.entries()) {
    if (!keys.has(secret.key_ref)) {
      refinement.addIssue({ code: "custom", message: "Secret entry references an unknown key", path: ["encrypted_secret_provider", "secrets", index, "key_ref"] });
    }
  }
  const paths = [
    manifest.credential_catalog.path,
    ...manifest.encrypted_secret_provider.keys.map(entry => entry.path),
    ...manifest.encrypted_secret_provider.secrets.map(entry => entry.path),
    ...manifest.ca_bundles.entries.map(entry => entry.path)
  ];
  requireUnique(paths, ["paths"], refinement);
});

export interface ExternalMcpProductionIoManifestLoaderOptions {
  readonly expectedOwnerUid?: number;
  readonly maximumManifestBytes?: number;
  readonly signal?: AbortSignal;
}

export interface LoadedExternalMcpProductionIo {
  readonly io: ExternalMcpProductionIoConfig;
  readonly options: ExternalMcpProductionIoOptions;
}

export async function loadExternalMcpProductionIoManifest(
  config: ExternalMcpConfig,
  env: NodeJS.ProcessEnv,
  loaderOptions: ExternalMcpProductionIoManifestLoaderOptions = {}
): Promise<LoadedExternalMcpProductionIo | undefined> {
  if (!config.enabled) return undefined;
  const signal = loaderOptions.signal ?? new AbortController().signal;
  signal.throwIfAborted();
  try {
    const path = env.DIPOLE_AGENT_EXTERNAL_MCP_IO_MANIFEST?.trim();
    if (path === undefined || path === "" || !isAbsolute(path) || normalize(path) !== path) throw new Error("invalid path");
    const expectedOwnerUid = loaderOptions.expectedOwnerUid ?? process.getuid?.();
    if (expectedOwnerUid === undefined || !Number.isSafeInteger(expectedOwnerUid) || expectedOwnerUid < 0) {
      throw new Error("invalid owner");
    }
    const maximumManifestBytes = loaderOptions.maximumManifestBytes ?? 256 * 1024;
    if (!Number.isSafeInteger(maximumManifestBytes) || maximumManifestBytes < 128 || maximumManifestBytes > 1024 * 1024) {
      throw new Error("invalid maximum");
    }
    const raw = await readManifest(path, expectedOwnerUid, maximumManifestBytes);
    signal.throwIfAborted();
    const manifest = rawManifestSchema.parse(raw);
    const keys = Object.fromEntries(manifest.encrypted_secret_provider.keys.map(entry => [entry.key_ref, entry.path]));
    const secrets = Object.fromEntries(manifest.encrypted_secret_provider.secrets.map(entry => [
      entry.secret_ref,
      { keyRef: entry.key_ref, path: entry.path }
    ]));
    const caBundles = Object.fromEntries(manifest.ca_bundles.entries.map(entry => [entry.ca_bundle_ref, entry.path]));
    return {
      io: {
        credentialCatalogPath: manifest.credential_catalog.path,
        secretProvider: {
          providerId: manifest.encrypted_secret_provider.provider_id,
          keys,
          secrets
        },
        caBundles
      },
      options: {
        expectedOwnerUid,
        maximumCatalogBytes: manifest.credential_catalog.maximum_bytes,
        maximumSecretBytes: manifest.encrypted_secret_provider.maximum_secret_bytes,
        maximumCaBundleBytes: manifest.ca_bundles.maximum_bytes,
        connectTimeoutMs: manifest.tls.connect_timeout_ms
      }
    };
  } catch {
    if (signal.aborted) signal.throwIfAborted();
    throw new Error("External MCP production I/O manifest is unavailable");
  }
}

function requireUnique(values: readonly string[], path: PropertyKey[], refinement: z.RefinementCtx): void {
  if (new Set(values).size !== values.length) {
    refinement.addIssue({ code: "custom", message: "Manifest entries must be unique", path });
  }
}

async function readManifest(path: string, expectedOwnerUid: number, maximumBytes: number): Promise<unknown> {
  const parent = dirname(path);
  if (await realpath(parent) !== parent) throw new Error("unsafe parent");
  const parentStats = await lstat(parent);
  if (!parentStats.isDirectory() || (parentStats.uid !== 0 && parentStats.uid !== expectedOwnerUid) ||
      (parentStats.mode & 0o100) === 0 || (parentStats.mode & 0o022) !== 0) {
    throw new Error("unsafe parent");
  }
  const handle = await open(path, constants.O_RDONLY | constants.O_NOFOLLOW);
  try {
    const stats = await handle.stat();
    if (!stats.isFile() || stats.nlink !== 1 || (stats.uid !== 0 && stats.uid !== expectedOwnerUid) ||
        (stats.mode & 0o400) === 0 || (stats.mode & 0o177) !== 0 || stats.size < 2 || stats.size > maximumBytes) {
      throw new Error("unsafe manifest");
    }
    const buffer = Buffer.allocUnsafe(maximumBytes + 1);
    let total = 0;
    while (total < buffer.length) {
      const result = await handle.read(buffer, total, buffer.length - total, null);
      if (result.bytesRead === 0) break;
      total += result.bytesRead;
    }
    if (total < 2 || total > maximumBytes || !isUtf8(buffer.subarray(0, total))) throw new Error("invalid manifest");
    return JSON.parse(buffer.subarray(0, total).toString("utf8")) as unknown;
  } finally {
    await handle.close();
  }
}
