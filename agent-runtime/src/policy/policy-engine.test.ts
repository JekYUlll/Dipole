import { describe, expect, it } from "vitest";

import { executionContextSchema } from "../runtime/execution-context.js";
import { AgentPolicyDeniedError, PolicyEngine } from "./policy-engine.js";

const context = executionContextSchema.parse({
  tenantId: "dipole",
  principalUuid: "U100",
  agentUuid: "UAI",
  delegatedByUuid: "U100",
  taskId: "TASK-1",
  mode: "active",
  permissions: ["conversation.read", "message.write"],
  resourceScopes: [
    { resourceType: "conversation", resourceId: "group:G1", actions: ["read"] },
    { resourceType: "conversation", resourceId: "*", actions: ["list"] }
  ],
  approvedCapabilities: []
});

describe("PolicyEngine", () => {
  const policy = new PolicyEngine();

  it("allows an exact resource scope and an explicit wildcard", () => {
    expect(() => policy.authorize(context, {
      id: "conversation.read",
      risk: "read",
      requiredPermission: "conversation.read"
    }, { resourceType: "conversation", resourceId: "group:G1", action: "read" })).not.toThrow();

    expect(() => policy.authorize(context, {
      id: "conversation.list",
      risk: "read",
      requiredPermission: "conversation.read"
    }, { resourceType: "conversation", resourceId: "direct:U100:U200", action: "list" })).not.toThrow();
  });

  it("denies a resource outside the pinned scope", () => {
    expect(() => policy.authorize(context, {
      id: "conversation.read",
      risk: "read",
      requiredPermission: "conversation.read"
    }, { resourceType: "conversation", resourceId: "group:G2", action: "read" })).toThrow(AgentPolicyDeniedError);
  });

  it("denies all write and destructive capabilities in shadow mode", () => {
    const shadow = executionContextSchema.parse({
      ...context,
      mode: "shadow",
      resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["write"] }]
    });
    expect(() => policy.authorize(shadow, {
      id: "message.send",
      risk: "write",
      requiredPermission: "message.write"
    }, { resourceType: "conversation", resourceId: "group:G1", action: "write" })).toThrow(AgentPolicyDeniedError);
  });

  it("requires explicit approval for destructive capabilities", () => {
    expect(() => policy.authorize(context, {
      id: "message.delete",
      risk: "destructive",
      requiredPermission: "message.write"
    }, { resourceType: "conversation", resourceId: "group:G1", action: "read" })).toThrow(AgentPolicyDeniedError);
  });
});
