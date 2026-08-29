import { afterEach, describe, expect, it, vi } from "vitest";

import type { ExternalMcpCredentialBinding } from "./external-mcp-credential-catalog.js";
import {
  ExternalMcpSecretAccessError,
  createExternalMcpAuthProvider,
  type ExternalMcpSecretProvider
} from "./external-mcp-auth-provider.js";

const binding: ExternalMcpCredentialBinding = {
  tenantId: "dipole",
  credentialRef: "CRED-0123456789ABCDEF",
  credentialVersion: 3,
  providerId: "vault-prod",
  providerSecretRef: "SECRET-0123456789ABCDEF"
};

afterEach(() => vi.useRealTimers());

describe("external MCP Secret Provider AuthProvider adapter", () => {
  it("reloads exact fresh bytes for every request and wipes each source buffer", async () => {
    const sources = [Buffer.from("token-first"), Buffer.from("token-second")];
    const signals: AbortSignal[] = [];
    const read = vi.fn(async (received: ExternalMcpCredentialBinding, signal: AbortSignal) => {
      expect(received).toEqual(binding);
      signals.push(signal);
      return sources[read.mock.calls.length - 1]!;
    });
    const auth = createExternalMcpAuthProvider(binding, { read });

    await expect(auth.token()).resolves.toBe("token-first");
    await expect(auth.token()).resolves.toBe("token-second");
    expect(read).toHaveBeenCalledTimes(2);
    expect(signals.every(signal => !signal.aborted)).toBe(true);
    expect(sources.every(source => source.every(byte => byte === 0))).toBe(true);
  });

  it("does not expose an automatic unauthorized refresh hook", () => {
    const auth = createExternalMcpAuthProvider(binding, { read: async () => Buffer.from("token") });
    expect("onUnauthorized" in auth).toBe(false);
  });

  it("aborts a timed-out provider and reports a stable redacted code", async () => {
    vi.useFakeTimers();
    let providerSignal: AbortSignal | undefined;
    const provider: ExternalMcpSecretProvider = {
      read: async (_binding, signal) => {
        providerSignal = signal;
        return new Promise<Uint8Array>(() => undefined);
      }
    };
    const auth = createExternalMcpAuthProvider(binding, provider, { timeoutMs: 100 });
    const pending = auth.token();
    const assertion = expect(pending).rejects.toMatchObject({ code: "secret_timeout" });
    await vi.advanceTimersByTimeAsync(100);
    await assertion;
    expect(providerSignal?.aborted).toBe(true);
  });

  it("wipes bytes that arrive after the timeout", async () => {
    vi.useFakeTimers();
    const late = Buffer.from("late-token");
    let release: ((value: Uint8Array) => void) | undefined;
    const auth = createExternalMcpAuthProvider(binding, {
      read: async () => new Promise<Uint8Array>(resolve => { release = resolve; })
    }, { timeoutMs: 100 });
    const pending = auth.token();
    const assertion = expect(pending).rejects.toMatchObject({ code: "secret_timeout" });
    await vi.advanceTimersByTimeAsync(100);
    await assertion;
    release!(late);
    await vi.runAllTimersAsync();
    await Promise.resolve();
    expect(late.every(byte => byte === 0)).toBe(true);
  });

  it("redacts provider failures", async () => {
    const auth = createExternalMcpAuthProvider(binding, {
      read: async () => { throw new Error("vault returned sensitive-token-value"); }
    });
    let observed: unknown;
    try {
      await auth.token();
    } catch (error) {
      observed = error;
    }
    expect(observed).toBeInstanceOf(ExternalMcpSecretAccessError);
    expect(observed).toMatchObject({ code: "secret_unavailable" });
    expect((observed as Error).message).not.toContain("sensitive-token-value");
  });

  it.each([
    ["empty", Buffer.alloc(0)],
    ["oversized", Buffer.from("12345678901234567")],
    ["invalid UTF-8", Buffer.from([0xff, 0xfe])],
    ["control character", Buffer.from("token\nvalue")],
    ["space", Buffer.from("Bearer token")]
  ])("rejects and wipes %s secret bytes", async (_name, source) => {
    const auth = createExternalMcpAuthProvider(binding, { read: async () => source }, { maximumBytes: 16 });
    await expect(auth.token()).rejects.toMatchObject({ code: "secret_invalid" });
    expect(source.every(byte => byte === 0)).toBe(true);
  });

  it("accepts the RFC 6750 bearer token character set", async () => {
    const source = Buffer.from("Az09-._~+/==");
    const auth = createExternalMcpAuthProvider(binding, { read: async () => source });
    await expect(auth.token()).resolves.toBe("Az09-._~+/==");
    expect(source.every(byte => byte === 0)).toBe(true);
  });

  it("rejects unsafe adapter limits", () => {
    const provider: ExternalMcpSecretProvider = { read: async () => Buffer.from("token") };
    expect(() => createExternalMcpAuthProvider(binding, provider, { timeoutMs: 99 })).toThrow(/timeout/i);
    expect(() => createExternalMcpAuthProvider(binding, provider, { timeoutMs: 60_001 })).toThrow(/timeout/i);
    expect(() => createExternalMcpAuthProvider(binding, provider, { maximumBytes: 15 })).toThrow(/bytes/i);
    expect(() => createExternalMcpAuthProvider(binding, provider, { maximumBytes: 8193 })).toThrow(/bytes/i);
  });
});
