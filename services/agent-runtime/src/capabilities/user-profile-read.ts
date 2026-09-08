import { z } from "zod";

import type { AgentCapabilityRPCClient, UserProfileResult } from "./agent-capability-rpc.js";
import type { AgentCapability } from "./registry.js";
import type { ExecutionContext } from "../runtime/execution-context.js";

const inputSchema = z.object({}).strict();
const inputSchemaDescriptor = { type: "object", properties: {}, additionalProperties: false } as const;

// The subject is intentionally omitted from input. Core derives it from the
// trusted task identity, while the policy engine requires the same user scope.
export class UserProfileReadCapability implements AgentCapability<Record<string, never>, UserProfileResult> {
  readonly descriptor = { id: "user.profile.read", risk: "read" as const, requiredPermission: "user.profile.read", inputSchema: inputSchemaDescriptor };
  readonly inputSchema = inputSchema;

  constructor(private readonly client: Pick<AgentCapabilityRPCClient, "readUserProfile">) {}

  resolveResource(_input: Record<string, never>, context: ExecutionContext) {
    return { resourceType: "user", resourceId: context.principalUuid, action: "read" };
  }

  execute(_input: Record<string, never>, context: ExecutionContext): Promise<UserProfileResult> {
    const ownerScope = context.resourceScopes.some((scope) =>
      scope.resourceType === "user" && scope.resourceId === context.principalUuid && scope.actions.includes("read")
    );
    if (!ownerScope) throw new Error("Agent user profile read requires an owner-scoped user/read grant");
    return this.client.readUserProfile(context);
  }
}
