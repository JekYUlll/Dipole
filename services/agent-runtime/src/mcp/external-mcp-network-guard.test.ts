import { afterEach, describe, expect, it, vi } from "vitest";

import type { ExternalMcpProfile } from "./external-mcp-profile.js";
import {
  ExternalMcpNetworkError,
  createExternalMcpNetworkGuardedFetch,
  type ExternalMcpNetworkDispatcher
} from "./external-mcp-network-guard.js";

const profile: ExternalMcpProfile = {
  profileId: "github-prod",
  tenantId: "dipole",
  serverId: "github-mcp",
  endpoint: "https://mcp.example.com/rpc",
  credentialRef: "CRED-0123456789ABCDEF",
  credentialVersion: 3,
  allowedHosts: ["mcp.example.com"],
  allowedPorts: [443],
  dnsResolution: "public_only",
  tlsServerName: "mcp.example.com",
  caBundleRef: "CA-0123456789ABCDEF",
  allowedTools: ["search"]
};

afterEach(() => vi.useRealTimers());

describe("external MCP network guard", () => {
  it("passes every public DNS answer to a pinned dispatcher", async () => {
    const resolve = vi.fn(async () => [
      { address: "8.8.8.8", family: 4 as const },
      { address: "2606:4700:4700::1111", family: 6 as const }
    ]);
    const dispatch = vi.fn<ExternalMcpNetworkDispatcher["dispatch"]>(async () => ({
      response: new Response("ok", { status: 200 }),
      connectedAddress: "8.8.8.8"
    }));
    const guardedFetch = createExternalMcpNetworkGuardedFetch(profile, { resolve }, { dispatch });

    await expect(guardedFetch(profile.endpoint, { method: "POST", body: "{}" })).resolves.toMatchObject({ status: 200 });
    expect(resolve).toHaveBeenCalledWith("mcp.example.com", expect.any(AbortSignal));
    expect(dispatch).toHaveBeenCalledOnce();
    const dispatched = dispatch.mock.calls[0]![0];
    expect(dispatched).toMatchObject({
      addresses: [
        { address: "8.8.8.8", family: 4 },
        { address: "2606:4700:4700::1111", family: 6 }
      ],
      tlsServerName: "mcp.example.com"
    });
    expect(dispatched.request.redirect).toBe("manual");
  });

  it.each([
    "0.0.0.0", "10.0.0.1", "100.64.0.1", "127.0.0.1", "169.254.1.1", "172.16.0.1",
    "192.0.2.1", "192.88.99.1", "192.168.1.1", "198.18.0.1", "198.51.100.1", "203.0.113.1", "224.0.0.1",
    "::", "::1", "::ffff:8.8.8.8", "64:ff9b::808:808", "64:ff9b:1::808:808", "100::1",
    "2001::1", "2001:db8::1", "2002:0808:0808::1", "3fff::1", "5f00::1", "fc00::1", "fe80::1", "ff00::1"
  ])("rejects non-public DNS answer %s", async (address) => {
    const family = address.includes(":") ? 6 as const : 4 as const;
    const dispatch = vi.fn<ExternalMcpNetworkDispatcher["dispatch"]>();
    const guardedFetch = createExternalMcpNetworkGuardedFetch(
      profile,
      { resolve: async () => [{ address: "8.8.8.8", family: 4 }, { address, family }] },
      { dispatch }
    );

    await expect(guardedFetch(profile.endpoint)).rejects.toMatchObject({ code: "dns_non_public" });
    expect(dispatch).not.toHaveBeenCalled();
  });

  it("resolves every request and blocks a later rebinding answer", async () => {
    const resolve = vi.fn()
      .mockResolvedValueOnce([{ address: "8.8.8.8", family: 4 }])
      .mockResolvedValueOnce([{ address: "127.0.0.1", family: 4 }]);
    const dispatch = vi.fn(async () => ({ response: new Response("ok"), connectedAddress: "8.8.8.8" }));
    const guardedFetch = createExternalMcpNetworkGuardedFetch(profile, { resolve }, { dispatch });

    await expect(guardedFetch(profile.endpoint)).resolves.toBeInstanceOf(Response);
    await expect(guardedFetch(profile.endpoint)).rejects.toMatchObject({ code: "dns_non_public" });
    expect(resolve).toHaveBeenCalledTimes(2);
    expect(dispatch).toHaveBeenCalledOnce();
  });

  it("rejects redirect responses without following the Location", async () => {
    const dispatch = vi.fn(async () => ({
      response: new Response(null, {
        status: 302,
        headers: { location: "https://other.example.com/mcp" }
      }),
      connectedAddress: "8.8.8.8"
    }));
    const guardedFetch = createExternalMcpNetworkGuardedFetch(
      profile,
      { resolve: async () => [{ address: "8.8.8.8", family: 4 }] },
      { dispatch }
    );

    await expect(guardedFetch(profile.endpoint)).rejects.toMatchObject({ code: "redirect_denied" });
  });

  it.each([
    "http://mcp.example.com/rpc",
    "https://other.example.com/rpc",
    "https://mcp.example.com:444/rpc",
    "https://user@mcp.example.com/rpc",
    "https://mcp.example.com/rpc?target=other"
  ])("rejects request URL outside the exact profile boundary: %s", async (url) => {
    const resolve = vi.fn(async () => [{ address: "8.8.8.8", family: 4 as const }]);
    const dispatch = vi.fn<ExternalMcpNetworkDispatcher["dispatch"]>();
    const guardedFetch = createExternalMcpNetworkGuardedFetch(profile, { resolve }, { dispatch });

    await expect(guardedFetch(url)).rejects.toMatchObject({ code: "request_denied" });
    expect(resolve).not.toHaveBeenCalled();
    expect(dispatch).not.toHaveBeenCalled();
  });

  it("aborts a DNS timeout and returns a stable error", async () => {
    vi.useFakeTimers();
    let observedSignal: AbortSignal | undefined;
    const guardedFetch = createExternalMcpNetworkGuardedFetch(profile, {
      resolve: async (_hostname, signal) => {
        observedSignal = signal;
        return new Promise(() => undefined);
      }
    }, { dispatch: async () => ({ response: new Response("unexpected"), connectedAddress: "8.8.8.8" }) }, { timeoutMs: 100 });

    const pending = guardedFetch(profile.endpoint);
    const assertion = expect(pending).rejects.toMatchObject({ code: "dns_timeout" });
    await vi.advanceTimersByTimeAsync(100);
    await assertion;
    expect(observedSignal?.aborted).toBe(true);
  });

  it("propagates caller cancellation into DNS resolution", async () => {
    const controller = new AbortController();
    let observedSignal: AbortSignal | undefined;
    const guardedFetch = createExternalMcpNetworkGuardedFetch(profile, {
      resolve: async (_hostname, signal) => {
        observedSignal = signal;
        return new Promise((_resolve, reject) => {
          signal.addEventListener("abort", () => reject(signal.reason), { once: true });
        });
      }
    }, { dispatch: async () => ({ response: new Response("unexpected"), connectedAddress: "8.8.8.8" }) });

    const pending = guardedFetch(profile.endpoint, { signal: controller.signal });
    controller.abort(new Error("caller details"));
    await expect(pending).rejects.toMatchObject({ code: "request_cancelled" });
    expect(observedSignal?.aborted).toBe(true);
  });

  it("rejects unsafe DNS timeout limits", () => {
    const resolver = { resolve: async () => [{ address: "8.8.8.8", family: 4 as const }] };
    const dispatcher = { dispatch: async () => ({ response: new Response("ok"), connectedAddress: "8.8.8.8" }) };

    expect(() => createExternalMcpNetworkGuardedFetch(profile, resolver, dispatcher, { timeoutMs: 99 })).toThrow(/timeout/i);
    expect(() => createExternalMcpNetworkGuardedFetch(profile, resolver, dispatcher, { timeoutMs: 60_001 })).toThrow(/timeout/i);
  });

  it("redacts resolver and dispatcher failures", async () => {
    const resolverFailure = createExternalMcpNetworkGuardedFetch(profile, {
      resolve: async () => { throw new Error("resolver leaked internal-zone.example"); }
    }, { dispatch: async () => ({ response: new Response("unexpected"), connectedAddress: "8.8.8.8" }) });
    const dispatcherFailure = createExternalMcpNetworkGuardedFetch(profile, {
      resolve: async () => [{ address: "8.8.8.8", family: 4 }]
    }, { dispatch: async () => { throw new Error("TLS leaked secret endpoint"); } });

    await expectStableRedactedError(resolverFailure(profile.endpoint), "dns_unavailable", "internal-zone");
    await expectStableRedactedError(dispatcherFailure(profile.endpoint), "transport_unavailable", "secret endpoint");
  });

  it("rejects a dispatcher peer outside the approved DNS set", async () => {
    const guardedFetch = createExternalMcpNetworkGuardedFetch(profile, {
      resolve: async () => [{ address: "8.8.8.8", family: 4 }]
    }, {
      dispatch: async () => ({ response: new Response("unexpected"), connectedAddress: "1.1.1.1" })
    });

    await expect(guardedFetch(profile.endpoint)).rejects.toMatchObject({ code: "connection_mismatch" });
  });
});

async function expectStableRedactedError(
  pending: Promise<Response>,
  code: ExternalMcpNetworkError["code"],
  forbidden: string
): Promise<void> {
  let observed: unknown;
  try {
    await pending;
  } catch (error) {
    observed = error;
  }
  expect(observed).toBeInstanceOf(ExternalMcpNetworkError);
  expect(observed).toMatchObject({ code });
  expect((observed as Error).message).not.toContain(forbidden);
}
