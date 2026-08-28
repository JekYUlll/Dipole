import { describe, expect, it, vi } from "vitest";

import type { AgentCapabilityRPCClient } from "../capabilities/agent-capability-rpc.js";
import type { ExternalMcpDeploymentPlan } from "../mcp/external-mcp-deployment-composition.js";
import { loadShadowRuntimeConfig } from "../runtime/shadow-runtime.js";
import {
  createExternalMcpAgentCapabilityRPCResourceFactory,
  type ExternalMcpAgentCapabilityRPCFactory
} from "./external-mcp-agent-capability-rpc-resource.js";

describe("external MCP Agent Capability RPC resource", () => {
  it("stays lazy until an enabled startup plan requests the resource", () => {
    const createRPC = vi.fn<ExternalMcpAgentCapabilityRPCFactory>();

    createExternalMcpAgentCapabilityRPCResourceFactory(config(), createRPC);

    expect(createRPC).not.toHaveBeenCalled();
  });

  it("returns one authenticated client as the exact Core and Artifact authority", async () => {
    const client = {} as AgentCapabilityRPCClient;
    const close = vi.fn();
    const createRPC = vi.fn<ExternalMcpAgentCapabilityRPCFactory>(() => ({ client, close }));
    const runtimeConfig = config();
    const createResource = createExternalMcpAgentCapabilityRPCResourceFactory(runtimeConfig, createRPC);

    const resource = await createResource(deployment("dipole"), new AbortController().signal);

    expect(createRPC).toHaveBeenCalledWith(runtimeConfig);
    expect(resource.dependencies).toEqual({ core: client, artifacts: client });
    expect(resource.dependencies.core).toBe(resource.dependencies.artifacts);
    expect(Object.isFrozen(resource.dependencies)).toBe(true);
    await resource.close();
    await resource.close();
    expect(close).toHaveBeenCalledOnce();
  });

  it("rejects disabled RPC and cross-tenant Profiles before transport creation", async () => {
    const cases = [
      { runtime: config(false), deployment: deployment("dipole") },
      { runtime: config(true, "dipole"), deployment: deployment() },
      { runtime: config(true, "dipole"), deployment: deployment("another-tenant") },
      { runtime: config(true, "dipole"), deployment: deployment("dipole", "another-tenant") }
    ];

    for (const candidate of cases) {
      const createRPC = vi.fn<ExternalMcpAgentCapabilityRPCFactory>();
      const createResource = createExternalMcpAgentCapabilityRPCResourceFactory(candidate.runtime, createRPC);
      await expect(createResource(candidate.deployment, new AbortController().signal))
        .rejects.toThrow(/^External MCP Agent Capability RPC resource is unavailable$/);
      expect(createRPC).not.toHaveBeenCalled();
    }
  });

  it("preserves cancellation before construction without touching RPC", async () => {
    const controller = new AbortController();
    controller.abort(new Error("cancelled before RPC resource"));
    const createRPC = vi.fn<ExternalMcpAgentCapabilityRPCFactory>();
    const createResource = createExternalMcpAgentCapabilityRPCResourceFactory(config(), createRPC);

    await expect(createResource(deployment("dipole"), controller.signal))
      .rejects.toThrow(/cancelled before RPC resource/i);
    expect(createRPC).not.toHaveBeenCalled();
  });

  it("closes a constructed transport when cancellation wins after creation", async () => {
    const controller = new AbortController();
    const close = vi.fn();
    const createResource = createExternalMcpAgentCapabilityRPCResourceFactory(config(), () => {
      controller.abort(new Error("cancelled after RPC resource"));
      return { client: {} as AgentCapabilityRPCClient, close };
    });

    await expect(createResource(deployment("dipole"), controller.signal))
      .rejects.toThrow(/cancelled after RPC resource/i);
    expect(close).toHaveBeenCalledOnce();
  });

  it("returns fixed construction and rollback cleanup failures", async () => {
    const failedConstruction = createExternalMcpAgentCapabilityRPCResourceFactory(config(), () => {
      throw new Error("sensitive RPC target");
    });
    await expect(failedConstruction(deployment("dipole"), new AbortController().signal))
      .rejects.toThrow(/^External MCP Agent Capability RPC resource is unavailable$/);

    const controller = new AbortController();
    const failedCleanup = createExternalMcpAgentCapabilityRPCResourceFactory(config(), () => {
      controller.abort(new Error("sensitive cancellation"));
      return {
        client: {} as AgentCapabilityRPCClient,
        close: () => { throw new Error("sensitive close target"); }
      };
    });
    await expect(failedCleanup(deployment("dipole"), controller.signal))
      .rejects.toThrow(/^External MCP Agent Capability RPC resource cleanup failed$/);
  });

  it("memoizes explicit close failure without touching the transport twice", async () => {
    const close = vi.fn(() => { throw new Error("sensitive explicit close"); });
    const createResource = createExternalMcpAgentCapabilityRPCResourceFactory(config(), () => ({
      client: {} as AgentCapabilityRPCClient,
      close
    }));
    const resource = await createResource(deployment("dipole"), new AbortController().signal);

    await expect(resource.close()).rejects.toThrow(/^External MCP Agent Capability RPC resource cleanup failed$/);
    await expect(resource.close()).rejects.toThrow(/^External MCP Agent Capability RPC resource cleanup failed$/);
    expect(close).toHaveBeenCalledOnce();
  });
});

function config(enabled = true, tenantId = "dipole") {
  return loadShadowRuntimeConfig({
    DIPOLE_AGENT_TENANT_ID: tenantId,
    DIPOLE_AGENT_CAPABILITY_RPC_ENABLED: String(enabled),
    DIPOLE_AGENT_CAPABILITY_RPC_TARGET: "127.0.0.1:50061",
    DIPOLE_INTERNAL_RPC_SHARED_SECRET: "test-only-shared-secret"
  });
}

function deployment(...tenantIds: string[]): ExternalMcpDeploymentPlan {
  return {
    config: {
      enabled: true,
      profiles: tenantIds.map((tenantId, index) => ({
        profileId: `profile-${index}`,
        tenantId,
        serverId: `server-${index}`,
        endpoint: "https://example.com/mcp",
        credentialRef: `CRED-000000000000000${index}`,
        credentialVersion: 1,
        allowedHosts: ["example.com"],
        allowedPorts: [443],
        dnsResolution: "public_only" as const,
        tlsServerName: "example.com",
        caBundleRef: `CA-00000000000000000${index}`,
        allowedTools: ["get_issue"]
      }))
    }
  } as unknown as ExternalMcpDeploymentPlan;
}
