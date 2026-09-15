import { describe, expect, it, vi } from "vitest";

import { UserProfileReadCapability } from "./user-profile-read.js";
import type { ExecutionContext } from "../runtime/execution-context.js";

const context: ExecutionContext = {
  tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId: "T1", runId: "R1", mode: "active",
  permissions: ["user.profile.read"], resourceScopes: [{ resourceType: "user", resourceId: "U100", actions: ["read"] }], approvedCapabilities: []
};

describe("UserProfileReadCapability", () => {
  it("uses the task principal as the only resource scope", async () => {
    const getUserProfile = vi.fn().mockResolvedValue({ userId: "U100", nickname: "owner", avatar: "", signature: "", userType: 0, status: 1 });
    const capability = new UserProfileReadCapability({ getUserProfile });

    await expect(capability.execute({}, context)).resolves.toMatchObject({ userId: "U100" });
    expect(capability.resolveResource({}, context)).toEqual({ resourceType: "user", resourceId: "U100", action: "read" });
    expect(getUserProfile).toHaveBeenCalledWith(context);
  });
});
