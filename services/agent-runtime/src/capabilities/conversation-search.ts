import { z } from "zod";

import type { ExecutionContext } from "../runtime/execution-context.js";
import type { AgentCapabilityRPCClient, ConversationSearchResult } from "./agent-capability-rpc.js";
import type { AgentCapability } from "./registry.js";

const inputSchema = z.object({
  query: z.string().trim().min(1).max(512),
  limit: z.number().int().min(1).max(50).default(10)
}).strict();

type ConversationSearchInput = z.infer<typeof inputSchema>;

export class ConversationSearchCapability implements AgentCapability<ConversationSearchInput, readonly ConversationSearchResult[]> {
  readonly descriptor = {
    id: "conversation.search", risk: "read" as const, requiredPermission: "conversation.search",
    inputSchema: {
      type: "object", properties: {
        query: { type: "string", minLength: 1, maxLength: 512 },
        limit: { type: "integer", minimum: 1, maximum: 50, default: 10 }
      }, required: ["query"], additionalProperties: false
    }
  } as const;
  readonly inputSchema = inputSchema;

  constructor(private readonly client: Pick<AgentCapabilityRPCClient, "searchConversations">) {}

  resolveResource() {
    return { resourceType: "conversation", resourceId: "*", action: "search" };
  }

  execute(input: ConversationSearchInput, context: ExecutionContext): Promise<readonly ConversationSearchResult[]> {
    return this.client.searchConversations(context, input.query, input.limit);
  }
}
