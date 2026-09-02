import { createHash } from "node:crypto";
import { z } from "zod";

import type { InputSchema } from "../capabilities/registry.js";
import type { AgentMcpToolCommandBeginResult } from "../capabilities/agent-capability-rpc.js";
import { PolicyEngine, type CapabilityDescriptor, type ResourceRequest } from "../policy/policy-engine.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import type { McpToolInvocationBegin } from "./mcp-tool-invocation.js";
import {
  enforceMcpToolEgressPolicy,
  validateMcpToolEgressPolicy,
  type McpToolEgressPolicy
} from "./mcp-tool-client.js";
import { canonicalMcpJSON } from "./canonical-json.js";

const identitySchema = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/);
const coreIdentitySchema = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$/);
const routeBindingSchema = z.string().regex(/^[A-Za-z][A-Za-z0-9_.-]{0,63}$/);
const producerInputSchema = z.object({
  workflowStep: z.number().int().min(0).max(255),
  ordinal: z.number().int().min(0).max(255),
  capabilityId: identitySchema,
  arguments: z.unknown(),
  approvalId: coreIdentitySchema.optional()
}).strict();

export interface ExternalMcpCapabilityRoute<I> {
  readonly descriptor: CapabilityDescriptor;
  readonly inputSchema: InputSchema<I>;
  readonly profileId: string;
  readonly serverId: string;
  readonly toolName: string;
  readonly egressPolicy: McpToolEgressPolicy;
  resolveResource(input: I, context: ExecutionContext): ResourceRequest;
}

interface PreparedExternalMcpCapabilityRoute {
  readonly descriptor: CapabilityDescriptor;
  readonly profileId: string;
  readonly serverId: string;
  readonly toolName: string;
  readonly arguments: Readonly<Record<string, unknown>>;
}

export class ExternalMcpCapabilityRouteRegistry {
  readonly #routes = new Map<string, ExternalMcpCapabilityRoute<unknown>>();

  constructor(private readonly policy = new PolicyEngine()) {}

  register<I>(route: ExternalMcpCapabilityRoute<I>): void {
    const capabilityId = route.descriptor.id.trim();
    if (!identitySchema.safeParse(capabilityId).success || this.#routes.has(capabilityId)) {
      throw new Error("External MCP Capability route ID is invalid or duplicated");
    }
    for (const value of [route.profileId, route.serverId, route.toolName]) {
      if (!routeBindingSchema.safeParse(value).success) throw new Error("External MCP Capability route binding is invalid");
    }
    const egressPolicy = validateMcpToolEgressPolicy(route.toolName, route.egressPolicy);
    this.#routes.set(capabilityId, {
      descriptor: route.descriptor,
      inputSchema: route.inputSchema as InputSchema<unknown>,
      profileId: route.profileId,
      serverId: route.serverId,
      toolName: route.toolName,
      egressPolicy,
      resolveResource: (input, context) => route.resolveResource(input as I, context)
    });
  }

  prepare(capabilityId: string, rawArguments: unknown, context: ExecutionContext): PreparedExternalMcpCapabilityRoute {
    const route = this.#routes.get(capabilityId.trim());
    if (route === undefined) throw new Error("External MCP Capability route is unavailable");
    const input = route.inputSchema.parse(rawArguments);
    this.policy.authorize(context, route.descriptor, route.resolveResource(input, context));
    return {
      descriptor: route.descriptor,
      profileId: route.profileId,
      serverId: route.serverId,
      toolName: route.toolName,
      arguments: enforceMcpToolEgressPolicy(input as Readonly<Record<string, unknown>>, route.egressPolicy)
    };
  }

  workerEgressPolicies(capabilityId: string): Readonly<Record<string, Readonly<Record<string, McpToolEgressPolicy>>>> {
    const route = this.#routes.get(capabilityId.trim());
    if (route === undefined) throw new Error("External MCP Capability route is unavailable");
    return {
      [route.profileId]: {
        [route.toolName]: {
          allowedArgumentNames: [...route.egressPolicy.allowedArgumentNames],
          maximumBytes: route.egressPolicy.maximumBytes
        }
      }
    };
  }
}

export interface McpInvocationBeginClient {
  beginMcpToolCommand(input: McpToolInvocationBegin): Promise<AgentMcpToolCommandBeginResult>;
}

export interface McpInvocationProducerResult extends AgentMcpToolCommandBeginResult {
  readonly taskId: string;
  readonly runId: string;
}

export class TrustedMcpInvocationProducer {
  constructor(
    private readonly routes: ExternalMcpCapabilityRouteRegistry,
    private readonly core: McpInvocationBeginClient
  ) {}

  async produce(rawInput: unknown, context: ExecutionContext): Promise<McpInvocationProducerResult> {
    validateProducerContext(context);
    const input = producerInputSchema.parse(rawInput);
    const route = this.routes.prepare(input.capabilityId, input.arguments, context);
    if (route.descriptor.risk === "read" && input.approvalId !== undefined) {
      throw new Error("External MCP read Invocation cannot bind an Approval");
    }
    if (route.descriptor.risk !== "read" && input.approvalId === undefined) {
      throw new Error("External MCP write Invocation requires an Approval");
    }
    const invocationId = stableInvocationId(context, input.workflowStep, input.ordinal);
    const argumentsJson = canonicalMcpJSON(route.arguments);
    const result = await this.core.beginMcpToolCommand({
      invocationId,
      taskId: context.taskId,
      runId: context.runId,
      toolName: route.toolName,
      capabilityId: route.descriptor.id,
      argumentsSha256: sha256(argumentsJson),
      profileId: route.profileId,
      serverId: route.serverId,
      argumentsJson,
      ...(context.requestId === undefined ? {} : { requestId: context.requestId }),
      ...(context.traceId === undefined ? {} : { traceId: context.traceId }),
      ...(input.approvalId === undefined ? {} : { approvalId: input.approvalId })
    });
    if (result.invocationId !== invocationId) throw new Error("External MCP Invocation begin returned a conflicting ID");
    return { ...result, taskId: context.taskId, runId: context.runId };
  }
}

function validateProducerContext(context: ExecutionContext): void {
  for (const value of [context.taskId, context.runId]) {
    if (!coreIdentitySchema.safeParse(value).success) throw new Error("External MCP producer Task/Run binding is invalid");
  }
  for (const value of [context.tenantId, context.principalUuid, context.agentUuid]) {
    if (!identitySchema.safeParse(value).success) throw new Error("External MCP producer identity binding is invalid");
  }
  for (const value of [context.requestId, context.traceId]) {
    if (value !== undefined && !identitySchema.safeParse(value).success) {
      throw new Error("External MCP producer correlation binding is invalid");
    }
  }
}

function stableInvocationId(context: ExecutionContext, workflowStep: number, ordinal: number): string {
  return sha256(canonicalMcpJSON({
    schemaVersion: "dipole.mcp.invocation-id.v1",
    tenantId: context.tenantId,
    principalUserId: context.principalUuid,
    agentId: context.agentUuid,
    taskId: context.taskId,
    runId: context.runId,
    workflowStep,
    ordinal
  }));
}

function sha256(value: string): string {
  return createHash("sha256").update(value, "utf8").digest("hex");
}
