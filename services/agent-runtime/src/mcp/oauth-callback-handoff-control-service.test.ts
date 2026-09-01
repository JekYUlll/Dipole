import { describe, expect, it } from "vitest";
import { createOAuthCallbackHandoffControlService } from "./oauth-callback-handoff-control-service.js";

describe("OAuthCallbackHandoffControlService", () => {
  it("forwards only the Gateway-approved handoff ID and correlation to a fixed Runtime lease owner", async () => {
    let received: unknown;
    const executor = { async execute(value: unknown) { received = value; return "completed"; } };
    const service = createOAuthCallbackHandoffControlService(executor as never, "runtime-worker-1");
    await service.notifyHandoff({ handoffId: "a".repeat(22), requestId: "REQ-1", traceId: "TRACE-1" });
    expect(received).toEqual({ handoffId: "a".repeat(22), leaseOwner: "runtime-worker-1", requestId: "REQ-1", traceId: "TRACE-1" });
  });

  it("rejects unsafe process lease-owner configuration", () => {
    expect(() => createOAuthCallbackHandoffControlService({} as never, "unsafe owner")).toThrow("lease owner");
  });
});
