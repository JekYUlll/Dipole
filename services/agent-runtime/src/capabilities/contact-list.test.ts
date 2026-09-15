import { describe, expect, it, vi } from "vitest";

import { ContactListCapability } from "./contact-list.js";
import type { ExecutionContext } from "../runtime/execution-context.js";

const context: ExecutionContext = {
  tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId: "T1", runId: "R1", mode: "active",
  permissions: ["contact.list"], resourceScopes: [{ resourceType: "contact", resourceId: "*", actions: ["list"] }], approvedCapabilities: []
};

describe("ContactListCapability", () => {
  it("requests a bounded contact list through the trusted task context", async () => {
    const listContacts = vi.fn().mockResolvedValue([]);
    const capability = new ContactListCapability({ listContacts });
    const input = capability.inputSchema.parse({ limit: 10 });

    await expect(capability.execute(input, context)).resolves.toEqual([]);
    expect(capability.resolveResource(input, context)).toEqual({ resourceType: "contact", resourceId: "*", action: "list" });
    expect(listContacts).toHaveBeenCalledWith(context, 10);
  });

  it("rejects inputs outside the bounded list contract", () => {
    const capability = new ContactListCapability({ listContacts: vi.fn() });
    expect(() => capability.inputSchema.parse({ limit: 51 })).toThrow();
    expect(() => capability.inputSchema.parse({ ownerId: "U999" })).toThrow();
  });
});
