import { z } from "zod";

import type { ExecutionContext } from "../runtime/execution-context.js";
import type { AgentCapabilityRPCClient, ConversationReadResult } from "./agent-capability-rpc.js";
import type { AgentCapability } from "./registry.js";

const inputSchema = z.object({
  conversationId: z.string().trim().min(1).max(256),
  limit: z.number().int().min(1).max(100).default(20)
}).strict();
type ConversationReadInput = z.infer<typeof inputSchema>;
const inputSchemaDescriptor = {
  type: "object", properties: {
    conversationId: { type: "string", minLength: 1, maxLength: 256 },
    limit: { type: "integer", minimum: 1, maximum: 100, default: 20 }
  }, required: ["conversationId"], additionalProperties: false
} as const;

export class ConversationReadCapability implements AgentCapability<ConversationReadInput, ConversationReadResult> {
  readonly descriptor = { id: "conversation.read", risk: "read" as const, requiredPermission: "conversation.read", inputSchema: inputSchemaDescriptor };
  readonly inputSchema = inputSchema;

  constructor(private readonly client: Pick<AgentCapabilityRPCClient, "readConversation">) {}

  resolveResource(input: ConversationReadInput) {
    return { resourceType: "conversation", resourceId: input.conversationId, action: "read" };
  }

  execute(input: ConversationReadInput, context: ExecutionContext): Promise<ConversationReadResult> {
    return this.client.readConversation(context, input.conversationId, input.limit);
  }
}
