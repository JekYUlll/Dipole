import type { ExecutionContext } from "../runtime/execution-context.js";

export type CapabilityRisk = "read" | "write" | "destructive";

export interface CapabilityDescriptor {
  readonly id: string;
  readonly risk: CapabilityRisk;
  readonly requiredPermission: string;
  readonly approvalRequired?: boolean;
  /** A low-sensitivity JSON Schema summary safe to expose to the model. */
  readonly inputSchema?: Readonly<Record<string, unknown>>;
}

export interface ResourceRequest {
  readonly resourceType: string;
  readonly resourceId: string;
  readonly action: string;
}

export class AgentPolicyDeniedError extends Error {
  constructor(message: string) {
    super(`agent policy denied: ${message}`);
    this.name = "AgentPolicyDeniedError";
  }
}

export class PolicyEngine {
  authorize(context: ExecutionContext, descriptor: CapabilityDescriptor, resource: ResourceRequest): void {
    if (!descriptor.id.trim() || !descriptor.requiredPermission.trim()) {
      throw new AgentPolicyDeniedError("invalid capability descriptor");
    }
    if (!context.permissions.includes(descriptor.requiredPermission)) {
      throw new AgentPolicyDeniedError(`missing permission ${descriptor.requiredPermission}`);
    }
    if (context.mode === "shadow" && descriptor.risk !== "read") {
      throw new AgentPolicyDeniedError("shadow mode cannot execute write capabilities");
    }
    if ((descriptor.approvalRequired === true || descriptor.risk === "destructive") && !context.approvedCapabilities.some((id) => id === descriptor.id)) {
      throw new AgentPolicyDeniedError(`capability ${descriptor.id} requires approval`);
    }
    const allowed = context.resourceScopes.some((scope) =>
      scope.resourceType === resource.resourceType &&
      (scope.resourceId === "*" || scope.resourceId === resource.resourceId) &&
      (scope.actions.includes("*") || scope.actions.includes(resource.action))
    );
    if (!allowed) {
      throw new AgentPolicyDeniedError(`resource scope ${resource.resourceType}/${resource.resourceId} does not allow ${resource.action}`);
    }
  }
}
