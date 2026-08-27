import type { Transport } from "@modelcontextprotocol/client";
import { readFile } from "node:fs/promises";
import { describe, expect, it, vi } from "vitest";

import {
  ExternalMcpTransportRegistry,
  externalMcpProfileSchemaVersion,
  loadExternalMcpConfig,
  type ExternalMcpTransportFactory
} from "./external-mcp-profile.js";

const validProfile = {
  schema_version: "dipole.agent.external-mcp-profile.v1",
  profile_id: "github-prod",
  tenant_id: "dipole",
  server_id: "github-mcp",
  endpoint: "https://mcp.github.example/v1",
  credential: { ref: "CRED-0123456789ABCDEF", version: 3 },
  network_policy: {
    allowed_hosts: ["mcp.github.example"],
    allowed_ports: [443],
    dns_resolution: "public_only",
    tls_server_name: "mcp.github.example",
    ca_bundle_ref: "CA-0123456789ABCDEF"
  },
  allowed_tools: ["search_repositories", "read_issue"]
};

describe("external MCP profile boundary", () => {
  it("keeps the language-neutral contract aligned with the runtime version", async () => {
    const path = new URL("../../../contracts/agent-external-mcp/v1/profile.schema.json", import.meta.url);
    const schema = JSON.parse(await readFile(path, "utf8")) as {
      $id: string;
      "x-dipole-version": string;
      properties: { schema_version: { const: string } };
    };
    expect(schema.$id).toMatch(/agent-external-mcp\/v1\/profile\.schema\.json$/);
    expect(schema["x-dipole-version"]).toBe(externalMcpProfileSchemaVersion);
    expect(schema.properties.schema_version.const).toBe(externalMcpProfileSchemaVersion);
  });

  it("stays disabled by default and ignores residual profile text", () => {
    expect(loadExternalMcpConfig({ DIPOLE_AGENT_EXTERNAL_MCP_PROFILES: "not-json" })).toEqual({
      enabled: false,
      profiles: []
    });
  });

  it("parses a tenant-bound profile containing only opaque credential references", () => {
    const config = loadExternalMcpConfig({
      DIPOLE_AGENT_EXTERNAL_MCP_ENABLED: "true",
      DIPOLE_AGENT_EXTERNAL_MCP_PROFILES: JSON.stringify([validProfile])
    });

    expect(config).toEqual({
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
        allowedTools: ["search_repositories", "read_issue"]
      }]
    });
    expect(JSON.stringify(config)).not.toContain("secret-value");
  });

  it.each([
    ["cleartext HTTP", { endpoint: "http://mcp.github.example/v1" }],
    ["embedded credentials", { endpoint: "https://user:password@mcp.github.example/v1" }],
    ["query parameters", { endpoint: "https://mcp.github.example/v1?token=abc" }],
    ["IP literal", { endpoint: "https://127.0.0.1/v1", network_policy: { ...validProfile.network_policy, allowed_hosts: ["127.0.0.1"], tls_server_name: "127.0.0.1" } }],
    ["localhost", { endpoint: "https://localhost/v1", network_policy: { ...validProfile.network_policy, allowed_hosts: ["localhost"], tls_server_name: "localhost" } }],
    ["internal suffix", { endpoint: "https://mcp.service.internal/v1", network_policy: { ...validProfile.network_policy, allowed_hosts: ["mcp.service.internal"], tls_server_name: "mcp.service.internal" } }],
    ["host outside allowlist", { network_policy: { ...validProfile.network_policy, allowed_hosts: ["other.example"] } }],
    ["port outside allowlist", { endpoint: "https://mcp.github.example:8443/v1" }],
    ["TLS identity mismatch", { network_policy: { ...validProfile.network_policy, tls_server_name: "other.example" } }],
    ["duplicate tools", { allowed_tools: ["read_issue", "read_issue"] }],
    ["raw secret field", { credential: { ...validProfile.credential, value: "secret-value" } }]
  ])("rejects %s", (_name, override) => {
    expect(() => loadExternalMcpConfig({
      DIPOLE_AGENT_EXTERNAL_MCP_ENABLED: "true",
      DIPOLE_AGENT_EXTERNAL_MCP_PROFILES: JSON.stringify([{ ...validProfile, ...override }])
    })).toThrow();
  });

  it("requires profiles when explicitly enabled", () => {
    expect(() => loadExternalMcpConfig({ DIPOLE_AGENT_EXTERNAL_MCP_ENABLED: "true" })).toThrow(/profiles/i);
  });

  it("opens only an exact tenant-owned profile through the injected factory", async () => {
    const config = loadExternalMcpConfig({
      DIPOLE_AGENT_EXTERNAL_MCP_ENABLED: "true",
      DIPOLE_AGENT_EXTERNAL_MCP_PROFILES: JSON.stringify([validProfile])
    });
    const transport = {} as Transport;
    const connect = vi.fn(async () => transport);
    const factory: ExternalMcpTransportFactory = { connect };
    const registry = new ExternalMcpTransportRegistry(config, factory);

    await expect(registry.connect("github-prod", "other-tenant")).rejects.toThrow(/tenant/i);
    expect(connect).not.toHaveBeenCalled();
    await expect(registry.connect("github-prod", "dipole")).resolves.toBe(transport);
    expect(connect).toHaveBeenCalledWith(expect.objectContaining({
      tenantId: "dipole",
      credentialRef: "CRED-0123456789ABCDEF",
      dnsResolution: "public_only",
      tlsServerName: "mcp.github.example"
    }), undefined);
  });

  it("refuses every connection while the kill switch is disabled", async () => {
    const connect = vi.fn();
    const registry = new ExternalMcpTransportRegistry(
      loadExternalMcpConfig({ DIPOLE_AGENT_EXTERNAL_MCP_PROFILES: JSON.stringify([validProfile]) }),
      { connect }
    );
    await expect(registry.connect("github-prod", "dipole")).rejects.toThrow(/disabled/i);
    expect(connect).not.toHaveBeenCalled();
  });
});
