import type { AuthProvider } from "@modelcontextprotocol/client";

import type { ExternalMcpCredentialBinding } from "./external-mcp-credential-catalog.js";

export type ExternalMcpSecretAccessErrorCode = "secret_timeout" | "secret_unavailable" | "secret_invalid";

export class ExternalMcpSecretAccessError extends Error {
  readonly code: ExternalMcpSecretAccessErrorCode;

  constructor(code: ExternalMcpSecretAccessErrorCode) {
    super({
      secret_timeout: "External MCP secret read timed out",
      secret_unavailable: "External MCP secret is unavailable",
      secret_invalid: "External MCP secret is invalid"
    }[code]);
    this.name = "ExternalMcpSecretAccessError";
    this.code = code;
  }
}

export interface ExternalMcpSecretProvider {
  read(binding: ExternalMcpCredentialBinding, signal: AbortSignal): Promise<Uint8Array>;
}

export interface ExternalMcpAuthProviderOptions {
  readonly timeoutMs?: number;
  readonly maximumBytes?: number;
}

export function createExternalMcpAuthProvider(
  binding: ExternalMcpCredentialBinding,
  provider: ExternalMcpSecretProvider,
  options: ExternalMcpAuthProviderOptions = {}
): AuthProvider {
  const timeoutMs = options.timeoutMs ?? 2000;
  if (!Number.isSafeInteger(timeoutMs) || timeoutMs < 100 || timeoutMs > 60_000) {
    throw new Error("External MCP secret timeout must be between 100 and 60000 milliseconds");
  }
  const maximumBytes = options.maximumBytes ?? 4096;
  if (!Number.isSafeInteger(maximumBytes) || maximumBytes < 16 || maximumBytes > 8192) {
    throw new Error("External MCP secret maximum bytes must be between 16 and 8192");
  }
  const trustedBinding = Object.freeze({ ...binding });
  return {
    token: () => readBearerToken(trustedBinding, provider, timeoutMs, maximumBytes)
  };
}

const providerFailure = Symbol("external-mcp-secret-provider-failure");
const decoder = new TextDecoder("utf-8", { fatal: true });

async function readBearerToken(
  binding: ExternalMcpCredentialBinding,
  provider: ExternalMcpSecretProvider,
  timeoutMs: number,
  maximumBytes: number
): Promise<string> {
  const controller = new AbortController();
  const timeoutError = new ExternalMcpSecretAccessError("secret_timeout");
  let deadlineReached = false;
  let source: Uint8Array | undefined;
  const read = Promise.resolve()
    .then(() => provider.read(binding, controller.signal))
    .then((bytes: Uint8Array) => {
      if (deadlineReached) {
        wipeLateBytes(bytes);
        throw timeoutError;
      }
      return bytes;
    }, () => {
      throw providerFailure;
    });
  let timer: ReturnType<typeof setTimeout> | undefined;
  const deadline = new Promise<never>((_resolve, reject) => {
    timer = setTimeout(() => {
      deadlineReached = true;
      controller.abort(new Error("External MCP secret read timed out"));
      reject(timeoutError);
    }, timeoutMs);
    timer.unref?.();
  });

  try {
    source = await Promise.race([read, deadline]);
    if (!(source instanceof Uint8Array) || source.byteLength === 0 || source.byteLength > maximumBytes) {
      throw new ExternalMcpSecretAccessError("secret_invalid");
    }
    let token: string;
    try {
      token = decoder.decode(source);
    } catch {
      throw new ExternalMcpSecretAccessError("secret_invalid");
    }
    if (!/^[A-Za-z0-9\-._~+/]+={0,}$/.test(token)) {
      throw new ExternalMcpSecretAccessError("secret_invalid");
    }
    return token;
  } catch (error) {
    if (error instanceof ExternalMcpSecretAccessError) throw error;
    if (error === providerFailure) throw new ExternalMcpSecretAccessError("secret_unavailable");
    throw new ExternalMcpSecretAccessError("secret_unavailable");
  } finally {
    if (timer !== undefined) clearTimeout(timer);
    if (source !== undefined) wipeBytes(source);
  }
}

function wipeBytes(bytes: Uint8Array): void {
  try {
    bytes.fill(0);
  } catch {
    throw new ExternalMcpSecretAccessError("secret_invalid");
  }
}

function wipeLateBytes(value: unknown): void {
  if (!(value instanceof Uint8Array)) return;
  try {
    value.fill(0);
  } catch {
    // A detached provider buffer is already inaccessible to the adapter.
  }
}
