import { z } from "zod";

import type { AgentCapabilityRPCClient, ContactListItem } from "./agent-capability-rpc.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import type { AgentCapability } from "./registry.js";

const inputSchema = z.object({ limit: z.number().int().min(1).max(50).default(20) }).strict();
type ContactListInput = z.infer<typeof inputSchema>;

export class ContactListCapability implements AgentCapability<ContactListInput, readonly ContactListItem[]> {
  readonly descriptor = {
    id: "contact.list", risk: "read" as const, requiredPermission: "contact.list",
    inputSchema: { type: "object", properties: { limit: { type: "integer", minimum: 1, maximum: 50, default: 20 } }, additionalProperties: false }
  } as const;
  readonly inputSchema = inputSchema;

  constructor(private readonly client: Pick<AgentCapabilityRPCClient, "listContacts">) {}

  resolveResource(_input: ContactListInput, _context: ExecutionContext) {
    return { resourceType: "contact", resourceId: "*", action: "list" };
  }

  execute(input: ContactListInput, context: ExecutionContext): Promise<readonly ContactListItem[]> {
    return this.client.listContacts(context, input.limit);
  }
}
