import { createDecipheriv, timingSafeEqual } from "node:crypto";
import { constants } from "node:fs";
import { lstat, open, realpath } from "node:fs/promises";
import { dirname, isAbsolute, normalize } from "node:path";

import type { ExternalMcpSecretProvider } from "./external-mcp-auth-provider.js";
import type { ExternalMcpCredentialBinding } from "./external-mcp-credential-catalog.js";

const identifierPattern = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$/;
const credentialRefPattern = /^CRED-[A-Z0-9]{16,64}$/;
const secretRefPattern = /^SECRET-[A-Z0-9]{16,64}$/;
const keyRefPattern = /^KEY-[A-Z0-9]{16,64}$/;
const envelopeMagic = Buffer.from("DPMCP01", "ascii");
const nonceBytes = 12;
const authenticationTagBytes = 16;

export const externalMcpEncryptedSecretEnvelopeMagic = Uint8Array.from(envelopeMagic);

export interface EncryptedFileExternalMcpSecretProviderConfig {
  readonly providerId: string;
  readonly keys: Readonly<Record<string, string>>;
  readonly secrets: Readonly<Record<string, {
    readonly keyRef: string;
    readonly path: string;
  }>>;
}

export interface EncryptedFileExternalMcpSecretProviderOptions {
  readonly expectedOwnerUid?: number;
  readonly maximumSecretBytes?: number;
}

export function createEncryptedFileExternalMcpSecretProvider(
  config: EncryptedFileExternalMcpSecretProviderConfig,
  options: EncryptedFileExternalMcpSecretProviderOptions = {}
): ExternalMcpSecretProvider {
  const trusted = validateConfig(config);
  const expectedOwnerUid = options.expectedOwnerUid ?? process.getuid?.();
  if (expectedOwnerUid === undefined || !Number.isSafeInteger(expectedOwnerUid) || expectedOwnerUid < 0) {
    throw new Error("External MCP encrypted secret Provider requires a valid expected owner UID");
  }
  const maximumSecretBytes = options.maximumSecretBytes ?? 8192;
  if (!Number.isSafeInteger(maximumSecretBytes) || maximumSecretBytes < 16 || maximumSecretBytes > 8192) {
    throw new Error("External MCP encrypted secret maximum bytes must be between 16 and 8192");
  }

  return {
    async read(binding, signal) {
      signal.throwIfAborted();
      let key: Buffer | undefined;
      let plaintext: Buffer | undefined;
      let returned = false;
      try {
        validateBinding(binding, trusted.providerId);
        const secret = trusted.secrets.get(binding.providerSecretRef);
        if (secret === undefined) throw new Error("unknown secret");
        const keyPath = trusted.keys.get(secret.keyRef);
        if (keyPath === undefined) throw new Error("unknown key");

        key = await readSecureFile(keyPath, expectedOwnerUid, 32, 32, true);
        signal.throwIfAborted();
        const maximumEnvelopeBytes = envelopeMagic.length + nonceBytes + maximumSecretBytes + authenticationTagBytes;
        const envelope = await readSecureFile(
          secret.path,
          expectedOwnerUid,
          envelopeMagic.length + nonceBytes + 1 + authenticationTagBytes,
          maximumEnvelopeBytes,
          false
        );
        signal.throwIfAborted();
        plaintext = decryptEnvelope(envelope, key, binding, secret.keyRef, maximumSecretBytes);
        signal.throwIfAborted();
        returned = true;
        return plaintext;
      } catch {
        if (signal.aborted) signal.throwIfAborted();
        throw new Error("External MCP encrypted secret is unavailable");
      } finally {
        key?.fill(0);
        if (!returned) plaintext?.fill(0);
      }
    }
  };
}

function validateConfig(config: EncryptedFileExternalMcpSecretProviderConfig): {
  readonly providerId: string;
  readonly keys: ReadonlyMap<string, string>;
  readonly secrets: ReadonlyMap<string, { readonly keyRef: string; readonly path: string }>;
} {
  const keyEntries = Object.entries(config.keys);
  const secretEntries = Object.entries(config.secrets);
  if (!identifierPattern.test(config.providerId) || keyEntries.length === 0 || keyEntries.length > 256 ||
      secretEntries.length === 0 || secretEntries.length > 256) {
    throw new Error("External MCP encrypted secret Provider configuration is invalid");
  }
  const paths = new Set<string>();
  const keys = new Map<string, string>();
  for (const [keyRef, path] of keyEntries) {
    assertConfiguredPath(keyRefPattern.test(keyRef), path, paths);
    keys.set(keyRef, path);
  }
  const secrets = new Map<string, { readonly keyRef: string; readonly path: string }>();
  for (const [secretRef, secret] of secretEntries) {
    assertConfiguredPath(secretRefPattern.test(secretRef) && keyRefPattern.test(secret.keyRef), secret.path, paths);
    if (!keys.has(secret.keyRef)) throw new Error("External MCP encrypted secret Provider configuration is invalid");
    secrets.set(secretRef, { keyRef: secret.keyRef, path: secret.path });
  }
  return { providerId: config.providerId, keys, secrets };
}

function assertConfiguredPath(validReference: boolean, path: string, paths: Set<string>): void {
  if (!validReference || !isAbsolute(path) || normalize(path) !== path || paths.has(path)) {
    throw new Error("External MCP encrypted secret Provider configuration is invalid");
  }
  paths.add(path);
}

function validateBinding(binding: ExternalMcpCredentialBinding, providerId: string): void {
  if (!identifierPattern.test(binding.tenantId) || !credentialRefPattern.test(binding.credentialRef) ||
      !Number.isSafeInteger(binding.credentialVersion) || binding.credentialVersion <= 0 ||
      binding.providerId !== providerId || !secretRefPattern.test(binding.providerSecretRef)) {
    throw new Error("invalid binding");
  }
}

async function readSecureFile(
  path: string,
  expectedOwnerUid: number,
  minimumBytes: number,
  maximumBytes: number,
  privateFile: boolean
): Promise<Buffer> {
  const parent = dirname(path);
  if (await realpath(parent) !== parent) throw new Error("unsafe parent");
  const parentStats = await lstat(parent);
  if (!parentStats.isDirectory() || (parentStats.uid !== 0 && parentStats.uid !== expectedOwnerUid) ||
      (parentStats.mode & 0o022) !== 0) {
    throw new Error("unsafe parent");
  }
  const handle = await open(path, constants.O_RDONLY | constants.O_NOFOLLOW);
  let bytes: Buffer | undefined;
  let returned = false;
  try {
    const stats = await handle.stat();
    const unsafeMode = (stats.mode & 0o111) !== 0 ||
      (privateFile ? (stats.mode & 0o077) !== 0 : (stats.mode & 0o022) !== 0);
    if (!stats.isFile() || stats.nlink !== 1 || (stats.uid !== 0 && stats.uid !== expectedOwnerUid) || unsafeMode ||
        stats.size < minimumBytes || stats.size > maximumBytes) {
      throw new Error("unsafe file");
    }
    bytes = Buffer.allocUnsafe(maximumBytes + 1);
    let total = 0;
    while (total < bytes.length) {
      const result = await handle.read(bytes, total, bytes.length - total, null);
      if (result.bytesRead === 0) break;
      total += result.bytesRead;
    }
    if (total < minimumBytes || total > maximumBytes) throw new Error("invalid file size");
    returned = true;
    return bytes.subarray(0, total);
  } finally {
    try {
      await handle.close();
    } catch (error) {
      if (privateFile) bytes?.fill(0);
      throw error;
    }
    if (privateFile && !returned) bytes?.fill(0);
  }
}

function decryptEnvelope(
  envelope: Buffer,
  key: Buffer,
  binding: ExternalMcpCredentialBinding,
  keyRef: string,
  maximumSecretBytes: number
): Buffer {
  const magic = envelope.subarray(0, envelopeMagic.length);
  if (magic.length !== envelopeMagic.length || !timingSafeEqual(magic, envelopeMagic)) throw new Error("invalid envelope");
  const nonceStart = envelopeMagic.length;
  const ciphertextStart = nonceStart + nonceBytes;
  const tagStart = envelope.length - authenticationTagBytes;
  const ciphertextBytes = tagStart - ciphertextStart;
  if (ciphertextBytes < 1 || ciphertextBytes > maximumSecretBytes) throw new Error("invalid envelope");

  const decipher = createDecipheriv("aes-256-gcm", key, envelope.subarray(nonceStart, ciphertextStart), {
    authTagLength: authenticationTagBytes
  });
  decipher.setAAD(bindingAad(binding, keyRef));
  decipher.setAuthTag(envelope.subarray(tagStart));
  let pending: Buffer | undefined;
  let final: Buffer | undefined;
  try {
    pending = decipher.update(envelope.subarray(ciphertextStart, tagStart));
    final = decipher.final();
    if (final.length === 0) return pending;
    const plaintext = Buffer.concat([pending, final]);
    pending.fill(0);
    final.fill(0);
    return plaintext;
  } catch (error) {
    pending?.fill(0);
    final?.fill(0);
    throw error;
  }
}

function bindingAad(binding: ExternalMcpCredentialBinding, keyRef: string): Buffer {
  return Buffer.from([
    "dipole.agent.external-mcp-secret.v1",
    binding.tenantId,
    binding.credentialRef,
    String(binding.credentialVersion),
    binding.providerId,
    binding.providerSecretRef,
    keyRef
  ].join("\0"), "utf8");
}
