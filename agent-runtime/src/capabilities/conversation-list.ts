import { z } from "zod";

import type { ExecutionContext } from "../runtime/execution-context.js";
import type { AgentCapabilityRPCClient } from "./agent-capability-rpc.js";
import type { AgentCapability } from "./registry.js";

const inputSchema = z.object({ limit: z.number().int().min(1).max(100).default(20) }).strict();
type ConversationListInput = z.infer<typeof inputSchema>;
const inputSchemaDescriptor = {
  type: "object", properties: { limit: { type: "integer", minimum: 1, maximum: 100, default: 20 } },
  additionalProperties: false
} as const;

export class ConversationListCapability implements AgentCapability<ConversationListInput, unknown> {
  readonly descriptor = { id: "conversation.list", risk: "read" as const, requiredPermission: "conversation.list", inputSchema: inputSchemaDescriptor };
  readonly inputSchema = inputSchema;

  constructor(private readonly client: Pick<AgentCapabilityRPCClient, "listConversations">) {}

  resolveResource() {
    return { resourceType: "conversation", resourceId: "*", action: "list" };
  }

  execute(input: ConversationListInput, context: ExecutionContext): Promise<unknown> {
    return this.client.listConversations(context, input.limit);
  }
}
