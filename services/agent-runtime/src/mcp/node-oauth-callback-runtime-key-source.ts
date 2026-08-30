import { createPrivateKey } from "node:crypto";
import { constants } from "node:fs";
import { lstat, open, realpath } from "node:fs/promises";
import { dirname, isAbsolute, normalize } from "node:path";

const keyIDPattern = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$/;

export interface OAuthCallbackRuntimeKeySource {
  use<T>(runtimeKeyId: string, operation: (privateKeyPEM: Buffer) => Promise<T> | T): Promise<T>;
}

export interface EncryptedFileOAuthCallbackRuntimeKeySourceConfig {
  readonly keys: Readonly<Record<string, string>>;
}

export interface EncryptedFileOAuthCallbackRuntimeKeySourceOptions {
  readonly expectedOwnerUid?: number;
}

/**
 * Opens only a configured Runtime private key for the duration of one operation.
 * This source is intentionally unmounted from the default Agent Runtime.
 */
export function createEncryptedFileOAuthCallbackRuntimeKeySource(
  config: EncryptedFileOAuthCallbackRuntimeKeySourceConfig,
  options: EncryptedFileOAuthCallbackRuntimeKeySourceOptions = {}
): OAuthCallbackRuntimeKeySource {
  const keys = validateConfig(config);
  const expectedOwnerUid = options.expectedOwnerUid ?? process.getuid?.();
  if (expectedOwnerUid === undefined || !Number.isSafeInteger(expectedOwnerUid) || expectedOwnerUid < 0) {
    throw new Error("OAuth callback Runtime key source requires a valid expected owner UID");
  }
  return {
    async use<T>(runtimeKeyId: string, operation: (privateKeyPEM: Buffer) => Promise<T> | T): Promise<T> {
      if (!keyIDPattern.test(runtimeKeyId) || typeof operation !== "function") throw new Error("OAuth callback Runtime key is unavailable");
      const path = keys.get(runtimeKeyId);
      if (path === undefined) throw new Error("OAuth callback Runtime key is unavailable");
      let key: Buffer | undefined;
      try {
        key = await readPrivateKey(path, expectedOwnerUid);
        validateRSAKey(key);
        return await operation(key);
      } catch {
        throw new Error("OAuth callback Runtime key is unavailable");
      } finally { key?.fill(0); }
    }
  };
}

function validateConfig(config: EncryptedFileOAuthCallbackRuntimeKeySourceConfig): ReadonlyMap<string, string> {
  const entries = Object.entries(config.keys);
  if (entries.length < 1 || entries.length > 32) throw new Error("OAuth callback Runtime key source configuration is invalid");
  const values = new Set<string>(); const keys = new Map<string, string>();
  for (const [id, path] of entries) {
    if (!keyIDPattern.test(id) || !isAbsolute(path) || normalize(path) !== path || values.has(path)) throw new Error("OAuth callback Runtime key source configuration is invalid");
    values.add(path); keys.set(id, path);
  }
  return keys;
}

async function readPrivateKey(path: string, expectedOwnerUid: number): Promise<Buffer> {
  const parent = dirname(path);
  if (await realpath(parent) !== parent) throw new Error("unsafe parent");
  const parentStats = await lstat(parent);
  if (!parentStats.isDirectory() || (parentStats.uid !== 0 && parentStats.uid !== expectedOwnerUid) || (parentStats.mode & 0o022) !== 0) throw new Error("unsafe parent");
  const handle = await open(path, constants.O_RDONLY | constants.O_NOFOLLOW);
  let key: Buffer | undefined;
  let returned = false;
  try {
    const stats = await handle.stat();
    if (!stats.isFile() || stats.nlink !== 1 || (stats.uid !== 0 && stats.uid !== expectedOwnerUid) || (stats.mode & 0o077) !== 0 || stats.size < 256 || stats.size > 8192) throw new Error("unsafe private key");
    key = Buffer.allocUnsafe(stats.size);
    const result = await handle.read(key, 0, key.length, 0);
    if (result.bytesRead !== key.length) throw new Error("short private key read");
    returned = true;
    return key;
  } finally {
    await handle.close();
    if (!returned) key?.fill(0);
  }
}

function validateRSAKey(key: Buffer): void {
  const privateKey = createPrivateKey({ key, format: "pem", type: "pkcs8" });
  const details = privateKey.asymmetricKeyDetails;
  if (privateKey.asymmetricKeyType !== "rsa" || details?.modulusLength === undefined || details.modulusLength < 2048) throw new Error("invalid Runtime private key");
}
