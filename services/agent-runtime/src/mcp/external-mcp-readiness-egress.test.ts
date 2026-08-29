import type { Transport } from "@modelcontextprotocol/client";
import { describe, expect, it, vi } from "vitest";

import type { AgentMCPReadinessEvidenceResolution } from "../capabilities/agent-capability-rpc.js";
import type { ExternalMcpConfig } from "./external-mcp-profile.js";
import type { ExternalMcpProductionIoConfig } from "./external-mcp-production-io.js";
import {
  ExternalMcpReadinessGatedTransportRegistry,
  type ExternalMcpFreshReadinessResolver
} from "./external-mcp-readiness-egress.js";
import {
  externalMcpReadinessBindingSha256,
  externalMcpReadinessProfileBindingSha256
} from "./external-mcp-readiness-evidence.js";

describe("external MCP readiness egress gate", () => {
  it("derives exact host bindings and authorizes immediately before the delegated connect", async () => {
    const order: string[] = [];
    const config = enabledConfig();
    const io = productionIo();
    const profileBinding = externalMcpReadinessProfileBindingSha256(config.profiles[0]!);
    const runtimeBinding = externalMcpReadinessBindingSha256(config, io, bindingOptions());
    const resolveFreshMcpReadinessEvidence = vi.fn(async () => {
      order.push("resolve");
      return receipt(profileBinding, runtimeBinding);
    });
    const connect = vi.fn(async () => {
      order.push("connect");
      return transport();
    });
    const registry = new ExternalMcpReadinessGatedTransportRegistry(
      config,
      io,
      underlying(connect),
      { resolveFreshMcpReadinessEvidence },
      bindingOptions()
    );

    await expect(registry.connect("github-prod", "dipole")).resolves.toBeDefined();

    expect(resolveFreshMcpReadinessEvidence).toHaveBeenCalledWith("dipole", profileBinding, runtimeBinding);
    expect(connect).toHaveBeenCalledWith("github-prod", "dipole", undefined);
    expect(order).toEqual(["resolve", "connect"]);
  });

  it("does not cache a prior receipt across connections", async () => {
    const config = enabledConfig();
    const io = productionIo();
    const resolver = resolverFor(config, io);
    const connect = vi.fn(async () => transport());
    const registry = new ExternalMcpReadinessGatedTransportRegistry(
      config, io, underlying(connect), resolver, bindingOptions()
    );

    await registry.connect("github-prod", "dipole");
    await registry.connect("github-prod", "dipole");

    expect(resolver.resolveFreshMcpReadinessEvidence).toHaveBeenCalledTimes(2);
    expect(connect).toHaveBeenCalledTimes(2);
  });

  it("fails closed before delegated connect when fresh evidence is absent or conflicting", async () => {
    const config = enabledConfig();
    const io = productionIo();
    const profileBinding = externalMcpReadinessProfileBindingSha256(config.profiles[0]!);
    const runtimeBinding = externalMcpReadinessBindingSha256(config, io, bindingOptions());
    const connect = vi.fn(async () => transport());
    const cases: Array<AgentMCPReadinessEvidenceResolution | undefined> = [
      undefined,
      { ...receipt(profileBinding, runtimeBinding), profileBindingSha256: "f".repeat(64) },
      { ...receipt(profileBinding, runtimeBinding), runtimeBindingSha256: "f".repeat(64) },
      { ...receipt(profileBinding, runtimeBinding), evidenceId: "invalid" },
      { ...receipt(profileBinding, runtimeBinding), expiresAt: new Date(Date.now() - 1).toISOString() }
    ];

    for (const resolved of cases) {
      const registry = new ExternalMcpReadinessGatedTransportRegistry(
        config,
        io,
        underlying(connect),
        { resolveFreshMcpReadinessEvidence: vi.fn(async () => resolved) },
        bindingOptions()
      );
      await expect(registry.connect("github-prod", "dipole"))
        .rejects.toThrow("External MCP readiness authorization failed");
    }
    expect(connect).not.toHaveBeenCalled();
  });

  it("rejects an unknown tenant or Profile without querying Core or the transport registry", async () => {
    const config = enabledConfig();
    const resolver = resolverFor(config, productionIo());
    const describe = vi.fn(() => config.profiles[0]!);
    const connect = vi.fn(async () => transport());
    const registry = new ExternalMcpReadinessGatedTransportRegistry(
      config, productionIo(), { describe, connect }, resolver, bindingOptions()
    );

    await expect(registry.connect("github-prod", "other"))
      .rejects.toThrow("External MCP readiness authorization failed");
    await expect(registry.connect("unknown", "dipole"))
      .rejects.toThrow("External MCP readiness authorization failed");

    expect(resolver.resolveFreshMcpReadinessEvidence).not.toHaveBeenCalled();
    expect(describe).not.toHaveBeenCalled();
    expect(connect).not.toHaveBeenCalled();
  });

  it("rejects underlying Profile drift after authorization and preserves later transport failures", async () => {
    const config = enabledConfig();
    const io = productionIo();
    const resolver = resolverFor(config, io);
    const connect = vi.fn(async () => transport());
    const drifted = new ExternalMcpReadinessGatedTransportRegistry(
      config,
      io,
      {
        describe: () => ({ ...config.profiles[0]!, credentialVersion: 4 }),
        connect
      },
      resolver,
      bindingOptions()
    );
    await expect(drifted.connect("github-prod", "dipole"))
      .rejects.toThrow("External MCP readiness authorization failed");
    expect(connect).not.toHaveBeenCalled();

    const unavailable = new ExternalMcpReadinessGatedTransportRegistry(
      config,
      io,
      underlying(vi.fn(async () => { throw new Error("transport unavailable"); })),
      resolverFor(config, io),
      bindingOptions()
    );
    await expect(unavailable.connect("github-prod", "dipole")).rejects.toThrow("transport unavailable");
  });

  it("preserves cancellation before lookup and after lookup without opening network state", async () => {
    const config = enabledConfig();
    const io = productionIo();
    const connect = vi.fn(async () => transport());
    const before = new AbortController();
    before.abort(new Error("cancelled before readiness"));
    const untouched = resolverFor(config, io);
    const beforeRegistry = new ExternalMcpReadinessGatedTransportRegistry(
      config, io, underlying(connect), untouched, bindingOptions()
    );

    await expect(beforeRegistry.connect("github-prod", "dipole", before.signal))
      .rejects.toThrow(/cancelled before readiness/i);
    expect(untouched.resolveFreshMcpReadinessEvidence).not.toHaveBeenCalled();

    const between = new AbortController();
    const resolved = resolverFor(config, io, () => between.abort(new Error("cancelled after readiness")));
    const betweenRegistry = new ExternalMcpReadinessGatedTransportRegistry(
      config, io, underlying(connect), resolved, bindingOptions()
    );
    await expect(betweenRegistry.connect("github-prod", "dipole", between.signal))
      .rejects.toThrow(/cancelled after readiness/i);
    expect(connect).not.toHaveBeenCalled();
  });

  it("sanitizes resolver failures before the transport boundary", async () => {
    const config = enabledConfig();
    const connect = vi.fn(async () => transport());
    const registry = new ExternalMcpReadinessGatedTransportRegistry(
      config,
      productionIo(),
      underlying(connect),
      { resolveFreshMcpReadinessEvidence: vi.fn(async () => { throw new Error("sensitive Core details"); }) },
      bindingOptions()
    );

    const error = await registry.connect("github-prod", "dipole").catch((caught: unknown) => caught);
    expect(String(error)).toBe("Error: External MCP readiness authorization failed");
    expect(String(error)).not.toMatch(/sensitive/i);
    expect(connect).not.toHaveBeenCalled();
  });
});

function resolverFor(
  config: Extract<ExternalMcpConfig, { enabled: true }>,
  io: ExternalMcpProductionIoConfig,
  beforeReturn: () => void = () => undefined
): ExternalMcpFreshReadinessResolver & { resolveFreshMcpReadinessEvidence: ReturnType<typeof vi.fn> } {
  const profileBinding = externalMcpReadinessProfileBindingSha256(config.profiles[0]!);
  const runtimeBinding = externalMcpReadinessBindingSha256(config, io, bindingOptions());
  return {
    resolveFreshMcpReadinessEvidence: vi.fn(async () => {
      beforeReturn();
      return receipt(profileBinding, runtimeBinding);
    })
  };
}

function receipt(profileBindingSha256: string, runtimeBindingSha256: string): AgentMCPReadinessEvidenceResolution {
  const now = Date.now();
  return {
    evidenceId: "e".repeat(64),
    profileBindingSha256,
    runtimeBindingSha256,
    contentSha256: "c".repeat(64),
    collectedAt: new Date(now - 60_000).toISOString(),
    expiresAt: new Date(now + 30 * 60_000).toISOString()
  };
}

function enabledConfig(): Extract<ExternalMcpConfig, { enabled: true }> {
  return {
    enabled: true,
    profiles: [{
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
    }]
  };
}

function productionIo(): ExternalMcpProductionIoConfig {
  return {
    credentialCatalogPath: "/run/dipole/catalog.json",
    secretProvider: {
      providerId: "local-aes-gcm",
      keys: { "KEY-0123456789ABCDEF": "/run/dipole/key.bin" },
      secrets: {
        "SECRET-0123456789ABCDEF": {
          keyRef: "KEY-0123456789ABCDEF",
          path: "/run/dipole/secret.bin"
        }
      }
    },
    caBundles: { "CA-0123456789ABCDEF": "/run/dipole/ca.pem" }
  };
}

function bindingOptions() {
  return { expectedOwnerUid: 1000, trustedTransportBuilder: true } as const;
}

function underlying(connect: (profileId: string, tenantId: string, signal?: AbortSignal) => Promise<Transport>) {
  return {
    describe: () => enabledConfig().profiles[0]!,
    connect
  };
}

function transport(): Transport {
  return { close: vi.fn() } as unknown as Transport;
}
