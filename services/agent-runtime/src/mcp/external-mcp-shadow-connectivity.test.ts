import { StreamableHTTPClientTransport, type Tool, type Transport } from "@modelcontextprotocol/client";
import { createMcpHandler, Server } from "@modelcontextprotocol/server";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  createExternalMcpShadowConnectivityDrill,
  externalMcpShadowConnectivityReceiptSchemaVersion,
  type ExternalMcpShadowConnectivityClient
} from "./external-mcp-shadow-connectivity.js";

const profile = {
  profileId: "github-prod",
  tenantId: "dipole",
  serverId: "github-mcp",
  allowedTools: ["read_issue", "list_issues"]
};
const transport = { close: vi.fn(), send: vi.fn(), start: vi.fn() } as unknown as Transport;

describe("external MCP Shadow connectivity drill", () => {
  beforeEach(() => vi.clearAllMocks());

  it("returns a low-sensitivity receipt after discovering every allowlisted Tool and closing", async () => {
    const registry = registryBoundary();
    const client = clientBoundary([tool("read_issue"), tool("list_issues")]);
    const createClient = vi.fn(() => client);
    const drill = createExternalMcpShadowConnectivityDrill(registry, {
      createClient,
      now: () => fixedNow(),
      requestTimeoutMs: 3_000
    });

    const receipt = await drill({ profileId: profile.profileId, tenantId: profile.tenantId });

    expect(receipt).toEqual({
      schemaVersion: externalMcpShadowConnectivityReceiptSchemaVersion,
      checkedAt: "2026-08-28T13:00:00.000Z",
      toolCount: 2
    });
    expect(registry.describe).toHaveBeenCalledWith(profile.profileId, profile.tenantId);
    expect(registry.connect).toHaveBeenCalledWith(profile.profileId, profile.tenantId, expect.any(AbortSignal));
    expect(createClient).toHaveBeenCalledWith({
      serverId: profile.serverId,
      allowedTools: profile.allowedTools,
      requestTimeoutMs: 3_000
    });
    expect(client.connect).toHaveBeenCalledWith(transport, expect.any(AbortSignal));
    expect(client.close).toHaveBeenCalledOnce();
    expect(transport.close).not.toHaveBeenCalled();
    expect(JSON.stringify(receipt)).not.toMatch(/github|read_issue|list_issues/i);
  });

  it("uses the default modern Client for initialize and discovery without invoking a Tool", async () => {
    const callTool = vi.fn(async () => ({ content: [{ type: "text" as const, text: "unexpected" }] }));
    const server = new Server({ name: profile.serverId, version: "1.0.0" }, { capabilities: { tools: {} } });
    server.setRequestHandler("tools/list", async () => ({
      tools: profile.allowedTools.map(name => ({ name, inputSchema: { type: "object" as const } }))
    }));
    server.setRequestHandler("tools/call", callTool);
    const handler = createMcpHandler(() => server);
    const clientTransport = new StreamableHTTPClientTransport(new URL("https://github.example/mcp"), {
      fetch: (url, init) => handler.fetch(new Request(url, init))
    });
    const registry = registryBoundary({ connect: async () => clientTransport });

    try {
      const drill = createExternalMcpShadowConnectivityDrill(registry, { now: () => fixedNow() });
      await expect(drill({ profileId: profile.profileId, tenantId: profile.tenantId }))
        .resolves.toMatchObject({ toolCount: 2 });
      expect(callTool).not.toHaveBeenCalled();
    } finally {
      await handler.close();
    }
  });

  it("fails closed when discovery omits an allowlisted Tool and still closes both owners", async () => {
    const registry = registryBoundary();
    const client = clientBoundary([tool("read_issue")]);
    const drill = createExternalMcpShadowConnectivityDrill(registry, { createClient: () => client });

    await expect(drill({ profileId: profile.profileId, tenantId: profile.tenantId }))
      .rejects.toThrow("External MCP Shadow connectivity drill failed");
    expect(client.close).toHaveBeenCalledOnce();
    expect(transport.close).not.toHaveBeenCalled();
  });

  it("withholds the receipt when post-discovery Client cleanup fails", async () => {
    const client = clientBoundary([tool("read_issue"), tool("list_issues")]);
    client.close.mockRejectedValueOnce(new Error("sensitive close failure"));
    const drill = createExternalMcpShadowConnectivityDrill(registryBoundary(), { createClient: () => client });

    const error = await drill({ profileId: profile.profileId, tenantId: profile.tenantId })
      .catch((caught: unknown) => caught);
    expect(String(error)).toBe("Error: External MCP Shadow connectivity drill failed");
    expect(String(error)).not.toMatch(/sensitive/);
  });

  it("maps Profile, Transport, handshake and clock failures to one low-sensitivity error", async () => {
    const cases = [
      {
        registry: registryBoundary({ describe: () => { throw new Error("sensitive profile"); } }),
        client: clientBoundary([]), now: () => fixedNow()
      },
      {
        registry: registryBoundary({ describe: () => ({ ...profile, profileId: "other-profile" }) }),
        client: clientBoundary([]), now: () => fixedNow()
      },
      {
        registry: registryBoundary({ connect: async () => { throw new Error("sensitive credential"); } }),
        client: clientBoundary([]), now: () => fixedNow()
      },
      {
        registry: registryBoundary(),
        client: clientBoundary([], async () => { throw new Error("sensitive server response"); }), now: () => fixedNow()
      },
      {
        registry: registryBoundary(), client: clientBoundary([]),
        now: () => { throw new Error("sensitive clock"); }
      }
    ];
    for (const candidate of cases) {
      const drill = createExternalMcpShadowConnectivityDrill(candidate.registry, {
        createClient: () => candidate.client,
        now: candidate.now
      });
      const error = await drill({ profileId: profile.profileId, tenantId: profile.tenantId })
        .catch((caught: unknown) => caught);
      expect(String(error)).toBe("Error: External MCP Shadow connectivity drill failed");
      expect(String(error)).not.toMatch(/sensitive/);
    }
  });

  it("propagates cancellation before access and during handshake, then cleans up", async () => {
    const before = new AbortController();
    before.abort(new Error("cancelled before drill"));
    const untouched = registryBoundary();
    const untouchedClient = clientBoundary([]);
    const preCancelled = createExternalMcpShadowConnectivityDrill(untouched, {
      createClient: () => untouchedClient
    });
    await expect(preCancelled({ profileId: profile.profileId, tenantId: profile.tenantId }, before.signal))
      .rejects.toThrow(/cancelled before drill/i);
    expect(untouched.describe).not.toHaveBeenCalled();

    const during = new AbortController();
    const client = clientBoundary([], async (_transport, signal) => {
      during.abort(new Error("cancelled during handshake"));
      signal.throwIfAborted();
      return [];
    });
    const drill = createExternalMcpShadowConnectivityDrill(registryBoundary(), { createClient: () => client });
    await expect(drill({ profileId: profile.profileId, tenantId: profile.tenantId }, during.signal))
      .rejects.toThrow(/cancelled during handshake/i);
    expect(client.close).toHaveBeenCalledOnce();
    expect(transport.close).toHaveBeenCalledOnce();
  });
});

function registryBoundary(overrides: Partial<{
  describe(profileId: string, tenantId: string): typeof profile;
  connect(profileId: string, tenantId: string, signal: AbortSignal): Promise<Transport>;
}> = {}) {
  return {
    describe: vi.fn(overrides.describe ?? (() => profile)),
    connect: vi.fn(overrides.connect ?? (async () => transport))
  };
}

function clientBoundary(
  tools: readonly Tool[],
  connect: (transport: Transport, signal: AbortSignal) => Promise<readonly Tool[]> = async () => tools
): ExternalMcpShadowConnectivityClient & { connect: ReturnType<typeof vi.fn>; close: ReturnType<typeof vi.fn> } {
  return {
    connect: vi.fn(connect),
    close: vi.fn(async () => undefined)
  };
}

function tool(name: string): Tool {
  return { name, inputSchema: { type: "object" } };
}

function fixedNow(): Date {
  return new Date("2026-08-28T13:00:00Z");
}
