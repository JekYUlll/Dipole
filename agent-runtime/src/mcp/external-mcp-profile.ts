import type { Transport } from "@modelcontextprotocol/client";
import { isIP } from "node:net";
import { z } from "zod";

import type { ExternalMcpCredentialBinding, ExternalMcpCredentialCatalog } from "./external-mcp-credential-catalog.js";

export const externalMcpProfileSchemaVersion = "dipole.agent.external-mcp-profile.v1" as const;

const identifierSchema = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$/);
const opaqueCredentialRefSchema = z.string().regex(/^CRED-[A-Z0-9]{16,64}$/);
const opaqueCaBundleRefSchema = z.string().regex(/^CA-[A-Z0-9]{16,64}$/);
const hostnameSchema = z.string().min(1).max(253);
const toolNameSchema = z.string().regex(/^[A-Za-z][A-Za-z0-9_.-]{0,63}$/);

const rawExternalMcpProfileSchema = z.object({
  schema_version: z.literal(externalMcpProfileSchemaVersion),
  profile_id: identifierSchema,
  tenant_id: identifierSchema,
  server_id: identifierSchema,
  endpoint: z.string().min(1).max(2048),
  credential: z.object({
    ref: opaqueCredentialRefSchema,
    version: z.number().int().positive()
  }).strict(),
  network_policy: z.object({
    allowed_hosts: z.array(hostnameSchema).min(1).max(16),
    allowed_ports: z.array(z.number().int().min(1).max(65_535)).min(1).max(16),
    dns_resolution: z.literal("public_only"),
    tls_server_name: hostnameSchema,
    ca_bundle_ref: opaqueCaBundleRefSchema
  }).strict(),
  allowed_tools: z.array(toolNameSchema).min(1).max(256)
}).strict().superRefine((profile, refinement) => {
  const allowedHosts = profile.network_policy.allowed_hosts;
  const allowedPorts = profile.network_policy.allowed_ports;
  const allowedTools = profile.allowed_tools;
  if (new Set(allowedHosts).size !== allowedHosts.length) {
    refinement.addIssue({ code: "custom", message: "MCP allowed hosts must be unique", path: ["network_policy", "allowed_hosts"] });
  }
  if (new Set(allowedPorts).size !== allowedPorts.length) {
    refinement.addIssue({ code: "custom", message: "MCP allowed ports must be unique", path: ["network_policy", "allowed_ports"] });
  }
  if (new Set(allowedTools).size !== allowedTools.length) {
    refinement.addIssue({ code: "custom", message: "MCP allowed Tools must be unique", path: ["allowed_tools"] });
  }
  for (const [index, hostname] of allowedHosts.entries()) {
    if (!isPublicExactHostname(hostname)) {
      refinement.addIssue({ code: "custom", message: "MCP allowed hosts require exact public DNS names", path: ["network_policy", "allowed_hosts", index] });
    }
  }
  if (!isPublicExactHostname(profile.network_policy.tls_server_name)) {
    refinement.addIssue({ code: "custom", message: "MCP TLS ServerName requires an exact public DNS name", path: ["network_policy", "tls_server_name"] });
  }

  let endpoint: URL;
  try {
    endpoint = new URL(profile.endpoint);
  } catch {
    refinement.addIssue({ code: "custom", message: "MCP endpoint must be an absolute HTTPS URL", path: ["endpoint"] });
    return;
  }
  if (endpoint.protocol !== "https:" || endpoint.username !== "" || endpoint.password !== "" || endpoint.search !== "" || endpoint.hash !== "") {
    refinement.addIssue({
      code: "custom",
      message: "MCP endpoint requires HTTPS without credentials, query, or fragment",
      path: ["endpoint"]
    });
  }
  const hostname = endpoint.hostname.toLowerCase();
  if (!isPublicExactHostname(hostname)) {
    refinement.addIssue({ code: "custom", message: "MCP endpoint requires a public DNS hostname", path: ["endpoint"] });
  }
  if (!allowedHosts.includes(hostname)) {
    refinement.addIssue({ code: "custom", message: "MCP endpoint host is outside the allowlist", path: ["endpoint"] });
  }
  const port = endpoint.port === "" ? 443 : Number(endpoint.port);
  if (!allowedPorts.includes(port)) {
    refinement.addIssue({ code: "custom", message: "MCP endpoint port is outside the allowlist", path: ["endpoint"] });
  }
  if (profile.network_policy.tls_server_name !== hostname) {
    refinement.addIssue({ code: "custom", message: "MCP TLS ServerName must exactly match the endpoint hostname", path: ["network_policy", "tls_server_name"] });
  }
});

export interface ExternalMcpProfile {
  readonly profileId: string;
  readonly tenantId: string;
  readonly serverId: string;
  readonly endpoint: string;
  readonly credentialRef: string;
  readonly credentialVersion: number;
  readonly allowedHosts: readonly string[];
  readonly allowedPorts: readonly number[];
  readonly dnsResolution: "public_only";
  readonly tlsServerName: string;
  readonly caBundleRef: string;
  readonly allowedTools: readonly string[];
}

export type ExternalMcpConfig =
  | { readonly enabled: false; readonly profiles: readonly [] }
  | { readonly enabled: true; readonly profiles: readonly ExternalMcpProfile[] };

export interface ExternalMcpTransportFactory {
  connect(input: {
    readonly profile: ExternalMcpProfile;
    readonly credential: ExternalMcpCredentialBinding;
  }, signal?: AbortSignal): Promise<Transport>;
}

export function loadExternalMcpConfig(env: NodeJS.ProcessEnv): ExternalMcpConfig {
  const enabled = parseBoolean(env.DIPOLE_AGENT_EXTERNAL_MCP_ENABLED, "DIPOLE_AGENT_EXTERNAL_MCP_ENABLED");
  if (!enabled) return { enabled: false, profiles: [] };

  const encodedProfiles = env.DIPOLE_AGENT_EXTERNAL_MCP_PROFILES?.trim();
  if (!encodedProfiles) throw new Error("Enabled external MCP requires profiles");
  let decodedProfiles: unknown;
  try {
    decodedProfiles = JSON.parse(encodedProfiles);
  } catch {
    throw new Error("External MCP profiles must be valid JSON");
  }
  const rawProfiles = z.array(rawExternalMcpProfileSchema).min(1).max(64).parse(decodedProfiles);
  if (new Set(rawProfiles.map(profile => profile.profile_id)).size !== rawProfiles.length) {
    throw new Error("External MCP profile IDs must be unique");
  }
  return {
    enabled: true,
    profiles: rawProfiles.map(profile => ({
      profileId: profile.profile_id,
      tenantId: profile.tenant_id,
      serverId: profile.server_id,
      endpoint: profile.endpoint,
      credentialRef: profile.credential.ref,
      credentialVersion: profile.credential.version,
      allowedHosts: [...profile.network_policy.allowed_hosts],
      allowedPorts: [...profile.network_policy.allowed_ports],
      dnsResolution: profile.network_policy.dns_resolution,
      tlsServerName: profile.network_policy.tls_server_name,
      caBundleRef: profile.network_policy.ca_bundle_ref,
      allowedTools: [...profile.allowed_tools]
    }))
  };
}

export class ExternalMcpTransportRegistry {
  readonly #config: ExternalMcpConfig;
  readonly #credentialCatalog: ExternalMcpCredentialCatalog;
  readonly #factory: ExternalMcpTransportFactory;
  readonly #now: () => Date;

  constructor(
    config: ExternalMcpConfig,
    credentialCatalog: ExternalMcpCredentialCatalog,
    factory: ExternalMcpTransportFactory,
    now: () => Date = () => new Date()
  ) {
    this.#config = config;
    this.#credentialCatalog = credentialCatalog;
    this.#factory = factory;
    this.#now = now;
  }

  async connect(profileId: string, tenantId: string, signal?: AbortSignal): Promise<Transport> {
    if (!this.#config.enabled) throw new Error("External MCP connections are disabled");
    signal?.throwIfAborted();
    const profile = this.#config.profiles.find(candidate => candidate.profileId === profileId);
    if (profile === undefined) throw new Error("External MCP profile is not configured");
    if (profile.tenantId !== tenantId) throw new Error("External MCP profile tenant does not match the execution tenant");
    const credential = await this.#credentialCatalog.resolve({
      tenantId,
      credentialRef: profile.credentialRef,
      credentialVersion: profile.credentialVersion,
      now: this.#now()
    });
    signal?.throwIfAborted();
    return this.#factory.connect({ profile, credential }, signal);
  }
}

function parseBoolean(raw: string | undefined, name: string): boolean {
  if (raw === undefined || raw.trim() === "") return false;
  const value = raw.trim().toLowerCase();
  if (value === "true") return true;
  if (value === "false") return false;
  throw new Error(`${name} must be true or false`);
}

function isPublicExactHostname(hostname: string): boolean {
  if (hostname !== hostname.toLowerCase() || hostname.endsWith(".") || isIP(hostname) !== 0) return false;
  if (!/^(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(hostname)) return false;
  return !hostname.endsWith(".local") && !hostname.endsWith(".internal") && !hostname.endsWith(".localhost");
}
