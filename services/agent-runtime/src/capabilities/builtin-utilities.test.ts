import { describe, expect, it } from "vitest";

import { CalculateCapability } from "./calculate.js";
import { GetCurrentTimeCapability } from "./get-current-time.js";
import { CapabilityRegistry } from "./registry.js";
import type { ExecutionContext } from "../runtime/execution-context.js";

const context: ExecutionContext = {
  tenantId: "dipole", principalUuid: "U1", agentUuid: "AI", taskId: "T1", runId: "R1", mode: "active",
  permissions: ["time.read", "calculator.evaluate"],
  resourceScopes: [
    { resourceType: "time", resourceId: "*", actions: ["read"] },
    { resourceType: "calculator", resourceId: "*", actions: ["evaluate"] }
  ],
  approvedCapabilities: []
};

function registry() {
  const value = new CapabilityRegistry();
  value.register(new GetCurrentTimeCapability(() => new Date("2026-09-15T12:34:56.000Z")));
  value.register(new CalculateCapability());
  return value;
}

describe("built-in utility capabilities", () => {
  it("returns a bounded current time in a validated IANA timezone", async () => {
    await expect(registry().execute("time.now", { timeZone: "Asia/Shanghai" }, context)).resolves.toEqual({
      iso: "2026-09-15T12:34:56.000Z", unixMs: 1789475696000, timeZone: "Asia/Shanghai"
    });
    await expect(registry().execute("time.now", { timeZone: "not/a-timezone" }, context)).rejects.toThrow(/time zone/i);
  });

  it("evaluates bounded arithmetic without evaluating source code", async () => {
    await expect(registry().execute("calculator.evaluate", { expression: "(12 + 3.5) * -2" }, context)).resolves.toEqual({
      expression: "(12 + 3.5) * -2", value: -31
    });
    await expect(registry().execute("calculator.evaluate", { expression: "process.exit()" }, context)).rejects.toThrow(/invalid token/i);
    await expect(registry().execute("calculator.evaluate", { expression: "4 / 0" }, context)).rejects.toThrow(/division by zero/i);
  });

  it("requires the matching permission and resource scope", async () => {
    await expect(registry().execute("time.now", {}, { ...context, permissions: ["calculator.evaluate"] })).rejects.toThrow(/time.read/);
    await expect(registry().execute("calculator.evaluate", { expression: "1+1" }, {
      ...context, resourceScopes: [{ resourceType: "time", resourceId: "*", actions: ["read"] }]
    })).rejects.toThrow(/calculator/);
  });
});
