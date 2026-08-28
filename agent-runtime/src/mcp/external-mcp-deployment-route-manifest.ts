import { isUtf8 } from "node:buffer";
import { createHash } from "node:crypto";
import { constants } from "node:fs";
import { lstat, open, realpath } from "node:fs/promises";
import { dirname, isAbsolute, normalize } from "node:path";
import { z } from "zod";

import type { InputSchema } from "../capabilities/registry.js";
import type { CapabilityDescriptor, ResourceRequest } from "../policy/policy-engine.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import type { TemporalMcpDispatchRoute } from "../temporal/mcp-dispatch-activity.js";
import type { ExternalMcpConfig } from "./external-mcp-profile.js";
import { canonicalMcpJSON } from "./canonical-json.js";
import {
  ExternalMcpCapabilityRouteRegistry
} from "./mcp-invocation-producer.js";
import {
  validateMcpToolEgressPolicy,
  type McpToolEgressPolicy
} from "./mcp-tool-client.js";

export const externalMcpDeploymentRouteManifestSchemaVersion =
  "dipole.agent.external-mcp-deployment-routes.v1" as const;

const identitySchema = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/);
const bindingSchema = z.string().regex(/^[A-Za-z][A-Za-z0-9_.-]{0,63}$/);
const argumentNameSchema = z.string().min(1).max(128);
const absolutePathSchema = z.string().min(1).max(4096).refine(path => isAbsolute(path) && normalize(path) === path);
const rawManifestSchema = z.object({
  schema_version: z.literal(externalMcpDeploymentRouteManifestSchemaVersion),
  routes: z.array(z.object({
    route_id: identitySchema,
    route_version: z.number().int().min(1).max(1_000_000),
    capability_id: identitySchema,
    workflow_step: z.number().int().min(0).max(255),
    ordinal: z.number().int().min(0).max(255),
    profile_id: bindingSchema,
    server_id: bindingSchema,
    tool_name: bindingSchema,
    egress_policy: z.object({
      allowed_argument_names: z.array(argumentNameSchema).max(256),
      maximum_bytes: z.number().int().min(2).max(64 * 1024)
    }).strict()
  }).strict()).min(1).max(256)
}).strict().superRefine((manifest, refinement) => {
  requireUnique(manifest.routes.map(route => route.route_id), ["routes", "route_id"], refinement);
  requireUnique(manifest.routes.map(route => route.capability_id), ["routes", "capability_id"], refinement);
  requireUnique(
    manifest.routes.map(route => `${route.workflow_step}:${route.ordinal}`),
    ["routes", "workflow_coordinate"],
    refinement
  );
});

export interface ExternalMcpCapabilityDefinition<I> {
  readonly descriptor: CapabilityDescriptor;
  readonly inputSchema: InputSchema<I>;
  readonly egressCeiling: McpToolEgressPolicy;
  resolveResource(input: I, context: ExecutionContext): ResourceRequest;
}

export interface ExternalMcpResolvedCapabilityDefinition {
  readonly descriptor: CapabilityDescriptor;
  readonly inputSchema: InputSchema<unknown>;
  readonly egressCeiling: McpToolEgressPolicy;
  resolveResource(input: unknown, context: ExecutionContext): ResourceRequest;
}

export class ExternalMcpCapabilityDefinitionRegistry {
  readonly #definitions = new Map<string, ExternalMcpResolvedCapabilityDefinition>();

  register<I>(definition: ExternalMcpCapabilityDefinition<I>): void {
    const capabilityId = definition.descriptor.id.trim();
    if (!identitySchema.safeParse(capabilityId).success || this.#definitions.has(capabilityId)) {
      throw new Error("External MCP Capability definition ID is invalid or duplicated");
    }
    if (!identitySchema.safeParse(definition.descriptor.requiredPermission).success ||
        !["read", "write", "destructive"].includes(definition.descriptor.risk) ||
        (definition.descriptor.approvalRequired !== undefined && typeof definition.descriptor.approvalRequired !== "boolean")) {
      throw new Error("External MCP Capability definition descriptor is invalid");
    }
    const egressCeiling = validateMcpToolEgressPolicy(capabilityId, definition.egressCeiling);
    this.#definitions.set(capabilityId, {
      descriptor: { ...definition.descriptor, id: capabilityId },
      inputSchema: definition.inputSchema as InputSchema<unknown>,
      egressCeiling,
      resolveResource: (input, context) => definition.resolveResource(input as I, context)
    });
  }

  resolve(capabilityId: string): ExternalMcpResolvedCapabilityDefinition | undefined {
    return this.#definitions.get(capabilityId);
  }
}

export interface ExternalMcpDeploymentRouteManifestLoaderOptions {
  readonly expectedOwnerUid?: number;
  readonly maximumManifestBytes?: number;
  readonly signal?: AbortSignal;
}

export interface LoadedExternalMcpDeploymentRoutes {
  readonly registry: ExternalMcpCapabilityRouteRegistry;
  readonly routes: readonly TemporalMcpDispatchRoute[];
}

export async function loadExternalMcpDeploymentRouteManifest(
  config: ExternalMcpConfig,
  definitions: ExternalMcpCapabilityDefinitionRegistry,
  env: NodeJS.ProcessEnv,
  options: ExternalMcpDeploymentRouteManifestLoaderOptions = {}
): Promise<LoadedExternalMcpDeploymentRoutes | undefined> {
  if (!config.enabled) return undefined;
  const signal = options.signal ?? new AbortController().signal;
  signal.throwIfAborted();
  try {
    const path = env.DIPOLE_AGENT_EXTERNAL_MCP_ROUTE_MANIFEST?.trim();
    if (path === undefined || !absolutePathSchema.safeParse(path).success) throw new Error("invalid path");
    const expectedOwnerUid = options.expectedOwnerUid ?? process.getuid?.();
    if (expectedOwnerUid === undefined || !Number.isSafeInteger(expectedOwnerUid) || expectedOwnerUid < 0) {
      throw new Error("invalid owner");
    }
    const maximumManifestBytes = options.maximumManifestBytes ?? 256 * 1024;
    if (!Number.isSafeInteger(maximumManifestBytes) || maximumManifestBytes < 128 || maximumManifestBytes > 1024 * 1024) {
      throw new Error("invalid maximum");
    }
    const raw = await readManifest(path, expectedOwnerUid, maximumManifestBytes);
    signal.throwIfAborted();
    const manifest = rawManifestSchema.parse(raw);
    const registry = new ExternalMcpCapabilityRouteRegistry();
    const routes = manifest.routes.map(route => {
      const definition = definitions.resolve(route.capability_id);
      if (definition === undefined) throw new Error("unknown definition");
      const profile = config.profiles.find(candidate => candidate.profileId === route.profile_id);
      if (profile === undefined || profile.serverId !== route.server_id || !profile.allowedTools.includes(route.tool_name)) {
        throw new Error("Profile route drift");
      }
      const egressPolicy = validateMcpToolEgressPolicy(route.tool_name, {
        allowedArgumentNames: route.egress_policy.allowed_argument_names,
        maximumBytes: route.egress_policy.maximum_bytes
      });
      const ceilingNames = new Set(definition.egressCeiling.allowedArgumentNames);
      if (egressPolicy.maximumBytes > definition.egressCeiling.maximumBytes ||
          egressPolicy.allowedArgumentNames.some(name => !ceilingNames.has(name))) {
        throw new Error("egress ceiling exceeded");
      }
      registry.register({
        descriptor: definition.descriptor,
        inputSchema: definition.inputSchema,
        profileId: profile.profileId,
        serverId: profile.serverId,
        toolName: route.tool_name,
        egressPolicy,
        resolveResource: definition.resolveResource
      });
      return {
        routeId: route.route_id,
        routeVersion: route.route_version,
        capabilityId: route.capability_id,
        workflowStep: route.workflow_step,
        ordinal: route.ordinal,
        deploymentBindingSha256: deploymentBindingSha256(route, definition)
      };
    });
    return { registry, routes };
  } catch {
    if (signal.aborted) signal.throwIfAborted();
    throw new Error("External MCP deployment route manifest is unavailable");
  }
}

function deploymentBindingSha256(
  route: z.infer<typeof rawManifestSchema>["routes"][number],
  definition: ExternalMcpResolvedCapabilityDefinition
): string {
  const binding = {
    schemaVersion: "dipole.agent.external-mcp-deployment-route-binding.v1",
    routeId: route.route_id,
    routeVersion: route.route_version,
    capability: {
      id: definition.descriptor.id,
      risk: definition.descriptor.risk,
      requiredPermission: definition.descriptor.requiredPermission,
      approvalRequired: definition.descriptor.approvalRequired ?? false
    },
    workflowStep: route.workflow_step,
    ordinal: route.ordinal,
    profileId: route.profile_id,
    serverId: route.server_id,
    toolName: route.tool_name,
    egressPolicy: {
      allowedArgumentNames: [...route.egress_policy.allowed_argument_names].sort(),
      maximumBytes: route.egress_policy.maximum_bytes
    }
  };
  return createHash("sha256").update(canonicalMcpJSON(binding), "utf8").digest("hex");
}

function requireUnique(values: readonly string[], path: PropertyKey[], refinement: z.RefinementCtx): void {
  if (new Set(values).size !== values.length) {
    refinement.addIssue({ code: "custom", message: "Deployment route manifest entries must be unique", path });
  }
}

async function readManifest(path: string, expectedOwnerUid: number, maximumBytes: number): Promise<unknown> {
  const parent = dirname(path);
  if (await realpath(parent) !== parent) throw new Error("unsafe parent");
  const parentStats = await lstat(parent);
  if (!parentStats.isDirectory() || (parentStats.uid !== 0 && parentStats.uid !== expectedOwnerUid) ||
      (parentStats.mode & 0o100) === 0 || (parentStats.mode & 0o022) !== 0) {
    throw new Error("unsafe parent");
  }
  const handle = await open(path, constants.O_RDONLY | constants.O_NOFOLLOW);
  try {
    const stats = await handle.stat();
    if (!stats.isFile() || stats.nlink !== 1 || (stats.uid !== 0 && stats.uid !== expectedOwnerUid) ||
        (stats.mode & 0o400) === 0 || (stats.mode & 0o177) !== 0 || stats.size < 2 || stats.size > maximumBytes) {
      throw new Error("unsafe manifest");
    }
    const buffer = Buffer.allocUnsafe(maximumBytes + 1);
    let total = 0;
    while (total < buffer.length) {
      const result = await handle.read(buffer, total, buffer.length - total, null);
      if (result.bytesRead === 0) break;
      total += result.bytesRead;
    }
    if (total < 2 || total > maximumBytes || !isUtf8(buffer.subarray(0, total))) throw new Error("invalid manifest");
    return JSON.parse(buffer.subarray(0, total).toString("utf8")) as unknown;
  } finally {
    await handle.close();
  }
}
