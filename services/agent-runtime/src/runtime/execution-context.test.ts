import { describe, expect, it } from "vitest";

import { executionContextSchema } from "./execution-context.js";

const context = {
  tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId: "TASK-1", runId: "RUN-1",
  mode: "active" as const, permissions: ["message.write"],
  resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["write"] }],
  approvedCapabilities: ["message.system.send"]
};

describe("ExecutionContext approved Capability projection", () => {
  it("accepts the explicit active Message Capability", () => {
    expect(executionContextSchema.parse(context).approvedCapabilities).toEqual(["message.system.send"]);
  });

  it("accepts the interactive assistant-reply Capability alongside system.send", () => {
    const parsed = executionContextSchema.parse({
      ...context,
      approvedCapabilities: ["message.system.send", "message.assistant_reply.send"]
    });
    expect(parsed.approvedCapabilities).toEqual(["message.system.send", "message.assistant_reply.send"]);
  });

  it("rejects unknown Capability IDs and Shadow approvals", () => {
    expect(() => executionContextSchema.parse({ ...context, approvedCapabilities: ["message.future.send"] })).toThrow();
    expect(() => executionContextSchema.parse({ ...context, mode: "shadow" })).toThrow();
  });
});
