import { z } from "zod";

import type { ExecutionContext } from "../runtime/execution-context.js";
import type { AgentCapabilityRPCClient, ConversationReadResult } from "./agent-capability-rpc.js";
import type { AgentCapability } from "./registry.js";

const inputSchema = z.object({
  targetId: z.string().trim().min(1).max(128),
  limit: z.number().int().min(1).max(100).default(20)
}).strict();
type ConversationReadInput = z.infer<typeof inputSchema>;

export class ConversationReadCapability implements AgentCapability<ConversationReadInput, ConversationReadResult> {
  readonly descriptor = { id: "conversation.read", risk: "read" as const, requiredPermission: "conversation.read" };
  readonly inputSchema = inputSchema;

  constructor(private readonly client: Pick<AgentCapabilityRPCClient, "readConversation">) {}

  // The Core RPC resolves target type and exact conversation scope authoritatively.
  resolveResource() {
    return { resourceType: "conversation", resourceId: "*", action: "read" };
  }

  execute(input: ConversationReadInput, context: ExecutionContext): Promise<ConversationReadResult> {
    return this.client.readConversation(context, input.targetId, input.limit);
  }
}
