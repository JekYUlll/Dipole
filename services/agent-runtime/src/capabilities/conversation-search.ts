import { z } from "zod";

import type { AgentCapabilityRPCClient, ConversationSearchEvidenceResult } from "./agent-capability-rpc.js";
import type { AgentCapability } from "./registry.js";
import type { ExecutionContext } from "../runtime/execution-context.js";

const inputSchema = z.object({
  query: z.string().trim().min(1).max(256),
  limit: z.number().int().min(1).max(20).default(10)
}).strict();
type ConversationSearchInput = z.infer<typeof inputSchema>;
const inputSchemaDescriptor = {
  type: "object", properties: {
    query: { type: "string", minLength: 1, maxLength: 256 },
    limit: { type: "integer", minimum: 1, maximum: 20, default: 10 }
  }, required: ["query"], additionalProperties: false
} as const;

// This remains opt-in at composition time. The capability itself does not give
// the Runtime a Search credential; Core resolves Task/Run authority per call.
export class ConversationSearchCapability implements AgentCapability<ConversationSearchInput, readonly ConversationSearchEvidenceResult[]> {
  readonly descriptor = { id: "conversation.search", risk: "read" as const, requiredPermission: "conversation.search", inputSchema: inputSchemaDescriptor };
  readonly inputSchema = inputSchema;

  constructor(private readonly client: Pick<AgentCapabilityRPCClient, "searchConversations">) {}

  resolveResource(_input: ConversationSearchInput, _context: ExecutionContext) {
    return { resourceType: "conversation", resourceId: "*", action: "read" };
  }

  execute(input: ConversationSearchInput, context: ExecutionContext): Promise<readonly ConversationSearchEvidenceResult[]> {
    return this.client.searchConversations(context, input.query, input.limit);
  }
}
