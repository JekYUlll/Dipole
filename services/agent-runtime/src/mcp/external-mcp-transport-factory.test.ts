import type {
  AuthProvider,
  StreamableHTTPClientTransportOptions,
  Transport
} from "@modelcontextprotocol/client";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/client";
import { describe, expect, it, vi } from "vitest";

import type { ExternalMcpCredentialBinding } from "./external-mcp-credential-catalog.js";
import type { ExternalMcpDnsResolver, ExternalMcpNetworkDispatcher } from "./external-mcp-network-guard.js";
import type { ExternalMcpProfile } from "./external-mcp-profile.js";
import {
  ExternalMcpTransportFactoryError,
  createExternalMcpStreamableHttpTransportFactory,
  type ExternalMcpStreamableHttpTransportBuilder
} from "./external-mcp-transport-factory.js";

const profile: ExternalMcpProfile = {
  profileId: "github-prod",
  tenantId: "dipole",
  serverId: "github-mcp",
  endpoint: "https://mcp.github.example/v1",
  credentialRef: "CRED-0123456789ABCDEF",
  credentialVersion: 3,
  allowedHosts: ["mcp.github.example"],
  allowedPorts: [443],
  dnsResolution: "public_only",
  tlsServerName: "mcp.github.example",
  caBundleRef: "CA-0123456789ABCDEF",
  allowedTools: ["read_issue"]
};

const credential: ExternalMcpCredentialBinding = {
  tenantId: "dipole",
  credentialRef: "CRED-0123456789ABCDEF",
  credentialVersion: 3,
  providerId: "vault-prod",
  providerSecretRef: "SECRET-0123456789ABCDEF"
};

function harness() {
  const secretBytes = new TextEncoder().encode("token-0123456789abcdef");
  const secretProvider = { read: vi.fn(async () => secretBytes) };
  const resolver: ExternalMcpDnsResolver = {
    resolve: vi.fn(async () => [{ address: "93.184.216.34", family: 4 as const }])
  };
  const dispatcher: ExternalMcpNetworkDispatcher = {
    dispatch: vi.fn(async () => ({
      response: new Response("ok", { status: 200 }),
      connectedAddress: "93.184.216.34"
    }))
  };
  const transports: Transport[] = [];
  const create = vi.fn((_url: URL, _options: StreamableHTTPClientTransportOptions) => {
    const transport = { close: vi.fn(), send: vi.fn(), start: vi.fn() } as unknown as Transport;
    transports.push(transport);
    return transport;
  });
  const builder: ExternalMcpStreamableHttpTransportBuilder = { create };
  const factory = createExternalMcpStreamableHttpTransportFactory({
    secretProvider,
    resolver,
    dispatcher,
    transportBuilder: builder
  });
  return { create, dispatcher, factory, resolver, secretBytes, secretProvider, transports };
}

describe("external MCP Streamable HTTP Transport Factory", () => {
  it("uses the official Streamable HTTP transport when no test builder is injected", async () => {
    const test = harness();
    const factory = createExternalMcpStreamableHttpTransportFactory({
      secretProvider: test.secretProvider,
      resolver: test.resolver,
      dispatcher: test.dispatcher
    });

    await expect(factory.connect({ profile, credential })).resolves.toBeInstanceOf(StreamableHTTPClientTransport);
  });

  it("binds a fresh official transport configuration to guarded fetch and fresh bearer reads", async () => {
    const test = harness();

    await expect(test.factory.connect({ profile, credential })).resolves.toBe(test.transports[0]);
    expect(test.create).toHaveBeenCalledOnce();
    const [url, options] = test.create.mock.calls[0]!;
    expect(url.href).toBe(profile.endpoint);
    expect(options).toMatchObject({
      onInsufficientScope: "throw",
      maxStepUpRetries: 0,
      reconnectionOptions: {
        initialReconnectionDelay: 1000,
        maxReconnectionDelay: 1000,
        reconnectionDelayGrowFactor: 1,
        maxRetries: 0
      }
    });
    expect(options.requestInit).toEqual({ redirect: "manual" });
    expect(options.authProvider).toBeDefined();
    expect(options.fetch).toBeDefined();

    await expect((options.authProvider as AuthProvider).token()).resolves.toBe("token-0123456789abcdef");
    expect(test.secretProvider.read).toHaveBeenCalledWith(credential, expect.any(AbortSignal));
    expect([...test.secretBytes]).toEqual(new Array(test.secretBytes.byteLength).fill(0));

    await expect(options.fetch!(profile.endpoint, { method: "POST" })).resolves.toBeInstanceOf(Response);
    expect(test.resolver.resolve).toHaveBeenCalledWith("mcp.github.example", expect.any(AbortSignal));
    expect(test.dispatcher.dispatch).toHaveBeenCalledWith(expect.objectContaining({
      addresses: [{ address: "93.184.216.34", family: 4 }],
      tlsServerName: "mcp.github.example",
      caBundleRef: "CA-0123456789ABCDEF"
    }), expect.any(AbortSignal));
  });

  it("creates independent auth providers and transports for every registry connection", async () => {
    const test = harness();

    const first = await test.factory.connect({ profile, credential });
    const second = await test.factory.connect({ profile, credential });

    expect(first).not.toBe(second);
    expect(test.create).toHaveBeenCalledTimes(2);
    expect(test.create.mock.calls[0]![1].authProvider).not.toBe(test.create.mock.calls[1]![1].authProvider);
    expect(test.create.mock.calls[0]![1].fetch).not.toBe(test.create.mock.calls[1]![1].fetch);
  });

  it("applies the reviewed Secret byte bound to every AuthProvider", async () => {
    const test = harness();
    const factory = createExternalMcpStreamableHttpTransportFactory({
      secretProvider: { read: vi.fn(async () => Buffer.from("12345678901234567")) },
      resolver: test.resolver,
      dispatcher: test.dispatcher,
      transportBuilder: { create: test.create },
      authProviderOptions: { maximumBytes: 16 }
    });
    await factory.connect({ profile, credential });
    const auth = test.create.mock.calls[0]![1].authProvider as AuthProvider;
    await expect(auth.token()).rejects.toMatchObject({ code: "secret_invalid" });
  });

  it.each([
    ["tenant", { tenantId: "other-tenant" }],
    ["reference", { credentialRef: "CRED-FEDCBA9876543210" }],
    ["version", { credentialVersion: 4 }]
  ])("rejects a %s binding mismatch before constructing network or secret state", async (_name, override) => {
    const test = harness();

    await expect(test.factory.connect({ credential: { ...credential, ...override }, profile }))
      .rejects.toMatchObject({ code: "credential_binding_mismatch" });
    expect(test.create).not.toHaveBeenCalled();
    expect(test.secretProvider.read).not.toHaveBeenCalled();
    expect(test.resolver.resolve).not.toHaveBeenCalled();
    expect(test.dispatcher.dispatch).not.toHaveBeenCalled();
  });

  it("rejects cancellation before constructing transport state", async () => {
    const test = harness();
    const controller = new AbortController();
    controller.abort(new Error("caller detail must stay private"));

    await expect(test.factory.connect({ profile, credential }, controller.signal))
      .rejects.toEqual(new ExternalMcpTransportFactoryError("setup_cancelled"));
    expect(test.create).not.toHaveBeenCalled();
  });

  it("redacts errors from the transport constructor", async () => {
    const test = harness();
    test.create.mockImplementationOnce(() => {
      throw new Error(`constructor leaked ${credential.providerSecretRef} ${profile.endpoint}`);
    });

    const error = await test.factory.connect({ profile, credential }).catch((caught: unknown) => caught);
    expect(error).toEqual(new ExternalMcpTransportFactoryError("transport_unavailable"));
    expect(String(error)).not.toContain(credential.providerSecretRef);
    expect(String(error)).not.toContain(profile.endpoint);
  });
});
