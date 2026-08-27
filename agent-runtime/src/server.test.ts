import { describe, expect, it, vi } from "vitest";

import { buildServer } from "./server.js";

describe("agent runtime health", () => {
  it("separates liveness from readiness", async () => {
    let ready = false;
    const server = buildServer({ isReady: () => ready });

    expect((await server.inject({ method: "GET", url: "/livez" })).statusCode).toBe(200);
    expect((await server.inject({ method: "GET", url: "/readyz" })).statusCode).toBe(503);
    ready = true;
    expect((await server.inject({ method: "GET", url: "/readyz" })).statusCode).toBe(200);
    await server.close();
  });
});

describe("agent runtime Task control API", () => {
  const headers = {
    "x-dipole-caller-service": "dipole-gateway",
    "x-dipole-service-token": "control-secret",
    "x-dipole-principal-user-id": "U100",
    "x-request-id": "R1",
    "x-trace-id": "T1"
  };

  it("requires the Gateway service identity and forwards only trusted principal headers", async () => {
    const getTask = vi.fn(async (input) => ({ taskId: input.taskId, status: "running" }));
    const server = buildServer({ isReady: () => true }, {
      secret: "control-secret", service: { getTask, cancelTask: vi.fn(), resolveApproval: vi.fn(), provideInput: vi.fn() }
    });
    expect((await server.inject({ method: "GET", url: "/internal/v1/agent/tasks/TASK-1" })).statusCode).toBe(401);
    expect((await server.inject({
      method: "GET", url: "/internal/v1/agent/tasks/TASK-1", headers: { ...headers, "x-dipole-service-token": "wrong" }
    })).statusCode).toBe(401);

    const response = await server.inject({ method: "GET", url: "/internal/v1/agent/tasks/TASK-1", headers });
    expect(response.statusCode).toBe(200);
    expect(getTask).toHaveBeenCalledWith({ taskId: "TASK-1", principalUserId: "U100", requestId: "R1", traceId: "T1" });
    await server.close();
  });

  it("binds cancellation, input, and approval to the authenticated principal", async () => {
    const cancelTask = vi.fn(async () => undefined);
    const resolveApproval = vi.fn(async () => undefined);
    const provideInput = vi.fn(async () => undefined);
    const server = buildServer({ isReady: () => true }, {
      secret: "control-secret", service: { getTask: vi.fn(), cancelTask, resolveApproval, provideInput }
    });
    const cancellation = await server.inject({
      method: "POST", url: "/internal/v1/agent/tasks/TASK-1/cancel", headers,
      payload: { reason: "user_cancelled", principalUserId: "U999" }
    });
    expect(cancellation.statusCode).toBe(202);
    expect(cancelTask).toHaveBeenCalledWith(expect.objectContaining({ taskId: "TASK-1", principalUserId: "U100", reason: "user_cancelled" }));

    const input = await server.inject({
      method: "POST", url: "/internal/v1/agent/tasks/TASK-1/inputs/INPUT-1", headers,
      payload: { value: { scope: "today" }, principalUserId: "U999" }
    });
    expect(input.statusCode).toBe(202);
    expect(provideInput).toHaveBeenCalledWith(expect.objectContaining({
      taskId: "TASK-1", principalUserId: "U100", requestId: "INPUT-1", value: { scope: "today" }
    }));

    const approval = await server.inject({
      method: "POST", url: "/internal/v1/agent/tasks/TASK-1/approvals/APR-1", headers,
      payload: { decision: "approved", principalUserId: "U999" }
    });
    expect(approval.statusCode).toBe(202);
    expect(resolveApproval).toHaveBeenCalledWith(expect.objectContaining({
      taskId: "TASK-1", principalUserId: "U100", approvalId: "APR-1", decision: "approved"
    }));
    await server.close();
  });
});
