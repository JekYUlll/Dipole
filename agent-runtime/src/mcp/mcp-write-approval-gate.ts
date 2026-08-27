import { createHash } from "node:crypto";

import type { CapabilityRegistry } from "../capabilities/registry.js";
import type { AgentCapabilityRPCClient } from "../capabilities/agent-capability-rpc.js";
import type { ResourceScope, ExecutionContext } from "../runtime/execution-context.js";
import { canonicalMcpJSON } from "./canonical-json.js";

export interface McpWriteApprovalGrant {
  readonly approvalId: string;
  readonly capabilityId: string;
  readonly resourceScope: ResourceScope;
  readonly scopeSha256: string;
  readonly argumentsSha256: string;
  readonly nonce: string;
}

export interface McpWriteApprovalGrantResolver {
  resolve(input: {
    readonly capabilityId: string;
    readonly resource: { readonly resourceType: string; readonly resourceId: string; readonly action: string };
    readonly arguments: unknown;
    readonly context: ExecutionContext;
  }): Promise<McpWriteApprovalGrant>;
}

export interface McpWriteApprovalConsumePort {
  consume(input: {
    readonly taskId: string;
    readonly runId: string;
    readonly approvalId: string;
    readonly capabilityId: string;
    readonly scopeSha256: string;
    readonly argumentsSha256: string;
    readonly nonceSha256: string;
    readonly requestId?: string;
    readonly traceId?: string;
  }): Promise<void>;
}

export class McpWriteApprovalGate {
  constructor(
    private readonly registry: CapabilityRegistry,
    private readonly approvals: McpWriteApprovalConsumePort,
    private readonly grants: McpWriteApprovalGrantResolver
  ) {}

  async execute(capabilityId: string, rawArguments: unknown, context: ExecutionContext): Promise<unknown> {
    if (context.mode !== "active") throw new Error("MCP write execution requires active mode");
    const prepared = this.registry.prepare(capabilityId, rawArguments, context);
    if (prepared.descriptor.risk === "read" || prepared.descriptor.approvalRequired !== true) {
      throw new Error("MCP write execution requires an approval-bound write Capability");
    }
    let grant: McpWriteApprovalGrant;
    try {
      grant = await this.grants.resolve({
        capabilityId: prepared.descriptor.id, resource: prepared.resource, arguments: prepared.input, context
      });
    } catch {
      throw new Error("MCP write Approval is unavailable");
    }
    const scope = {
      resourceType: prepared.resource.resourceType,
      resourceId: prepared.resource.resourceId,
      actions: [prepared.resource.action]
    };
    const scopeSha256 = resourceScopeSha256(scope);
    const argumentsSha256 = sha256(canonicalMcpJSON(prepared.input));
    if (
      !validGrant(grant)
      || grant.capabilityId !== prepared.descriptor.id
      || resourceScopeSha256(grant.resourceScope) !== scopeSha256
      || grant.scopeSha256 !== scopeSha256
      || grant.argumentsSha256 !== argumentsSha256
    ) {
      throw new Error("MCP write Approval binding does not match the invocation");
    }
    try {
      await this.approvals.consume({
        taskId: context.taskId,
        runId: context.runId,
        approvalId: grant.approvalId,
        capabilityId: prepared.descriptor.id,
        scopeSha256,
        argumentsSha256,
        nonceSha256: sha256(grant.nonce),
        ...(context.requestId === undefined ? {} : { requestId: context.requestId }),
        ...(context.traceId === undefined ? {} : { traceId: context.traceId })
      });
    } catch {
      throw new Error("MCP write Approval is unavailable or already consumed");
    }
    return prepared.execute();
  }
}

export function createMcpWriteApprovalConsumePort(
  client: Pick<AgentCapabilityRPCClient, "consumeApproval">
): McpWriteApprovalConsumePort {
  return {
    consume: input => client.consumeApproval(input.taskId, input.runId, {
      approvalId: input.approvalId,
      capabilityId: input.capabilityId,
      scopeSha256: input.scopeSha256,
      argumentsSha256: input.argumentsSha256,
      nonceSha256: input.nonceSha256
    }, {
      ...(input.requestId === undefined ? {} : { requestId: input.requestId }),
      ...(input.traceId === undefined ? {} : { traceId: input.traceId })
    })
  };
}

function validGrant(grant: McpWriteApprovalGrant): boolean {
  return /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/.test(grant.approvalId)
    && /^[A-Za-z][A-Za-z0-9_.-]{0,63}$/.test(grant.capabilityId)
    && /^[a-f0-9]{64}$/.test(grant.scopeSha256)
    && /^[a-f0-9]{64}$/.test(grant.argumentsSha256)
    && grant.nonce.length >= 16
    && grant.nonce.length <= 256
    && /^[\x21-\x7e]+$/.test(grant.nonce);
}

function resourceScopeSha256(scope: ResourceScope): string {
  const resourceType = scope.resourceType.trim();
  const resourceId = scope.resourceId.trim();
  const actions = scope.actions.map(action => action.trim()).sort();
  if (!resourceType || !resourceId || actions.length === 0 || actions.some(action => !action) || new Set(actions).size !== actions.length) {
    throw new Error("MCP write Approval resource scope is invalid");
  }
  return sha256(["dipole.agent.scope.v1", resourceType, resourceId, ...actions].join("\n"));
}

function sha256(value: string): string {
  return createHash("sha256").update(value, "utf8").digest("hex");
}
