import { describe, expect, it, vi } from "vitest";

import type { ExecutionContext } from "../runtime/execution-context.js";
import { CapabilityRegistry } from "./registry.js";
import { UserProfileReadCapability } from "./user-profile-read.js";

const context: ExecutionContext = {
  tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId: "TASK-1", runId: "RUN-1", mode: "shadow",
  permissions: ["user.profile.read"], resourceScopes: [{ resourceType: "user", resourceId: "U100", actions: ["read"] }], approvedCapabilities: []
};

describe("UserProfileReadCapability", () => {
  it("uses the trusted principal as the only readable user", async () => {
    const readUserProfile = vi.fn(async (received: ExecutionContext) => {
      expect(received).toBe(context);
      return { found: true, userId: "U100", nickname: "Ada", avatar: "", userType: 0, status: 1 };
    });
    const registry = new CapabilityRegistry();
    registry.register(new UserProfileReadCapability({ readUserProfile }));

    await expect(registry.execute("user.profile.read", {}, context)).resolves.toMatchObject({ userId: "U100" });
    expect(readUserProfile).toHaveBeenCalledOnce();
  });

  it("rejects a wildcard or another user's scope before remote execution", async () => {
    const readUserProfile = vi.fn();
    const registry = new CapabilityRegistry();
    registry.register(new UserProfileReadCapability({ readUserProfile }));
    const wildcard = { ...context, resourceScopes: [{ resourceType: "user", resourceId: "*", actions: ["read"] }] } as ExecutionContext;
    const otherUser = { ...context, resourceScopes: [{ resourceType: "user", resourceId: "U200", actions: ["read"] }] } as ExecutionContext;

    await expect(registry.execute("user.profile.read", {}, wildcard)).rejects.toThrow(/scope/);
    await expect(registry.execute("user.profile.read", {}, otherUser)).rejects.toThrow(/scope/);
    expect(readUserProfile).not.toHaveBeenCalled();
  });
});
