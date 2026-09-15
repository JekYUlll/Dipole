import { z } from "zod";

import type { AgentCapabilityRPCClient, UserProfile } from "./agent-capability-rpc.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import type { AgentCapability } from "./registry.js";

const inputSchema = z.object({}).strict();

export class UserProfileReadCapability implements AgentCapability<Record<string, never>, UserProfile> {
  readonly descriptor = {
    id: "user.profile.read", risk: "read" as const, requiredPermission: "user.profile.read",
    inputSchema: { type: "object", properties: {}, additionalProperties: false }
  } as const;
  readonly inputSchema = inputSchema;

  constructor(private readonly client: Pick<AgentCapabilityRPCClient, "getUserProfile">) {}

  resolveResource(_input: Record<string, never>, context: ExecutionContext) {
    return { resourceType: "user", resourceId: context.principalUuid, action: "read" };
  }

  execute(_input: Record<string, never>, context: ExecutionContext): Promise<UserProfile> {
    return this.client.getUserProfile(context);
  }
}
