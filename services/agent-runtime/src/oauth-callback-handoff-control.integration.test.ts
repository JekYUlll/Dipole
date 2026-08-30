import { describe, expect, it } from "vitest";

import { createOAuthCallbackHandoffControlService } from "./mcp/oauth-callback-handoff-control-service.js";
import { buildServer } from "./server.js";

describe("OAuth callback handoff control to executor", () => {
  it("accepts an authenticated notification once and forwards only the fixed lease request", async () => {
    const executions: unknown[] = [];
    const executor = { async execute(input: unknown) { executions.push(input); return "completed"; } };
    const server = buildServer({ isReady: () => true }, undefined, undefined, undefined, {
      secret: "control-secret",
      service: createOAuthCallbackHandoffControlService(executor as never, "runtime-worker-1")
    });
    const request = {
      method: "POST" as const,
      url: "/internal/v1/agent/oauth/callback-handoffs",
      headers: {
        "x-dipole-caller-service": "dipole-gateway", "x-dipole-service-token": "control-secret",
        "x-request-id": "REQ-1", "x-trace-id": "TRACE-1"
      },
      payload: { handoff_id: "a".repeat(22) }
    };
    expect((await server.inject(request)).statusCode).toBe(202);
    expect((await server.inject(request)).statusCode).toBe(202);
    expect(executions).toEqual([{ handoffId: "a".repeat(22), leaseOwner: "runtime-worker-1", requestId: "REQ-1", traceId: "TRACE-1" }]);
    await server.close();
  });
});
