import { isUtf8 } from "node:buffer";
import { X509Certificate } from "node:crypto";
import { constants } from "node:fs";
import { lstat, open, realpath } from "node:fs/promises";
import { dirname, isAbsolute, normalize } from "node:path";

const caBundleRefPattern = /^CA-[A-Z0-9]{16,64}$/;
const certificatePattern = /-----BEGIN CERTIFICATE-----[\s\S]+?-----END CERTIFICATE-----/g;

export interface ExternalMcpCaBundleProvider {
  read(caBundleRef: string, signal: AbortSignal): Promise<Uint8Array>;
}

export interface FileExternalMcpCaBundleProviderOptions {
  readonly expectedOwnerUid?: number;
  readonly maximumBytes?: number;
}

export function createFileExternalMcpCaBundleProvider(
  files: Readonly<Record<string, string>>,
  options: FileExternalMcpCaBundleProviderOptions = {}
): ExternalMcpCaBundleProvider {
  const entries = Object.entries(files);
  if (entries.length === 0 || entries.length > 256) throw new Error("External MCP CA bundle map is empty or too large");
  const paths = new Map<string, string>();
  for (const [reference, path] of entries) {
    if (!caBundleRefPattern.test(reference) || !isAbsolute(path) || normalize(path) !== path || paths.has(reference)) {
      throw new Error("External MCP CA bundle mapping is invalid");
    }
    paths.set(reference, path);
  }
  const expectedOwnerUid = options.expectedOwnerUid ?? process.getuid?.();
  if (expectedOwnerUid === undefined || !Number.isSafeInteger(expectedOwnerUid) || expectedOwnerUid < 0) {
    throw new Error("External MCP CA bundle provider requires a valid expected owner UID");
  }
  const maximumBytes = options.maximumBytes ?? 256 * 1024;
  if (!Number.isSafeInteger(maximumBytes) || maximumBytes < 256 || maximumBytes > 1024 * 1024) {
    throw new Error("External MCP CA bundle maximum size must be between 256 bytes and 1 MiB");
  }
  return {
    async read(caBundleRef, signal) {
      signal.throwIfAborted();
      const path = paths.get(caBundleRef);
      if (path === undefined) throw new Error("External MCP CA bundle is not configured");
      const contents = await loadCaBundle(path, expectedOwnerUid, maximumBytes);
      signal.throwIfAborted();
      validateExternalMcpCaBundle(contents, maximumBytes);
      return Uint8Array.from(contents);
    }
  };
}

export function validateExternalMcpCaBundle(contents: Uint8Array, maximumBytes = 256 * 1024): void {
  if (contents.byteLength < 256 || contents.byteLength > maximumBytes || !isUtf8(contents)) {
    throw new Error("External MCP CA bundle is outside its size or encoding bound");
  }
  const source = Buffer.from(contents).toString("utf8");
  const certificates = source.match(certificatePattern) ?? [];
  if (certificates.length === 0 || certificates.length > 32 || source.replace(certificatePattern, "").trim() !== "") {
    throw new Error("External MCP CA bundle must contain only PEM certificates");
  }
  try {
    for (const certificate of certificates) new X509Certificate(certificate);
  } catch {
    throw new Error("External MCP CA bundle contains an invalid certificate");
  }
}

async function loadCaBundle(path: string, expectedOwnerUid: number, maximumBytes: number): Promise<Buffer> {
  const parent = dirname(path);
  if (await realpath(parent) !== parent) throw new Error("External MCP CA bundle parent must be canonical without symlinks");
  const parentStats = await lstat(parent);
  if (!parentStats.isDirectory() || (parentStats.uid !== 0 && parentStats.uid !== expectedOwnerUid) ||
      (parentStats.mode & 0o022) !== 0) {
    throw new Error("External MCP CA bundle parent permissions are invalid");
  }
  const handle = await open(path, constants.O_RDONLY | constants.O_NOFOLLOW);
  try {
    const stats = await handle.stat();
    if (!stats.isFile() || stats.nlink !== 1 || (stats.uid !== 0 && stats.uid !== expectedOwnerUid) ||
        (stats.mode & 0o022) !== 0 || stats.size < 256 || stats.size > maximumBytes) {
      throw new Error("External MCP CA bundle file evidence is invalid");
    }
    const buffer = Buffer.allocUnsafe(maximumBytes + 1);
    let total = 0;
    while (total < buffer.length) {
      const result = await handle.read(buffer, total, buffer.length - total, null);
      if (result.bytesRead === 0) break;
      total += result.bytesRead;
    }
    if (total < 256 || total > maximumBytes) throw new Error("External MCP CA bundle size is outside its bound");
    return Buffer.from(buffer.subarray(0, total));
  } finally {
    await handle.close();
  }
}
