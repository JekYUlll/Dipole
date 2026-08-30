import { describe, expect, it, vi } from "vitest";

import { buildServer } from "./server.js";
import { SubscriptionShadowMetrics } from "./observability/subscription-shadow-metrics.js";

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

  it("mounts low-cardinality subscription Shadow metrics only when explicitly supplied", async () => {
    const disabled = buildServer({ isReady: () => true });
    expect((await disabled.inject({ method: "GET", url: "/metrics" })).statusCode).toBe(404);
    await disabled.close();

    const metrics = new SubscriptionShadowMetrics();
    metrics.observe({ directTargetAccepted: true, subscriptionOutcome: "match", candidateCount: 1 });
    const enabled = buildServer({ isReady: () => true }, undefined, undefined, metrics);
    const response = await enabled.inject({ method: "GET", url: "/metrics" });
    expect(response.statusCode).toBe(200);
    expect(response.headers["content-type"]).toContain("text/plain");
    expect(response.body).toContain("dipole_agent_subscription_shadow_comparisons_total");
    await enabled.close();
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

  it("creates an interactive Task only through the trusted Gateway identity", async () => {
    const startTask = vi.fn(async () => ({ taskId: "TASK-INTERACTIVE", status: "accepted" as const }));
    const server = buildServer({ isReady: () => true }, {
      secret: "control-secret", service: { startTask, getTask: vi.fn(), cancelTask: vi.fn(), resolveApproval: vi.fn(), provideInput: vi.fn() }
    });
    expect((await server.inject({ method: "POST", url: "/internal/v1/agent/tasks", payload: {} })).statusCode).toBe(401);
    const response = await server.inject({
      method: "POST", url: "/internal/v1/agent/tasks", headers,
      payload: { clientRequestId: "client-1", goal: "Summarize my unread work.", principalUserId: "U999" }
    });
    expect(response.statusCode).toBe(202);
    expect(response.json()).toEqual({ taskId: "TASK-INTERACTIVE", status: "accepted" });
    expect(startTask).toHaveBeenCalledWith({
      principalUserId: "U100", requestId: "R1", traceId: "T1",
      body: { clientRequestId: "client-1", goal: "Summarize my unread work.", principalUserId: "U999" }
    });
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

describe("agent runtime MCP HTTP API", () => {
  const headers = {
    "x-dipole-caller-service": "dipole-gateway",
    "x-dipole-service-token": "mcp-secret",
    "x-dipole-principal-user-id": "U100",
    "x-dipole-oauth-resource": "https://dipole.local/api/v1/agent/mcp",
    "x-dipole-oauth-scope": "dipole.agent.mcp.read",
    "x-request-id": "R1",
    "x-trace-id": "T1"
  };

  it("stays unmounted by default and requires the trusted Gateway identity", async () => {
    const fetch = vi.fn(async () => new Response("ok"));
    const disabled = buildServer({ isReady: () => true });
    expect((await disabled.inject({ method: "POST", url: "/internal/v1/agent/tasks/TASK-1/runs/RUN-1/mcp", payload: {} })).statusCode).toBe(404);
    await disabled.close();

    const server = buildServer({ isReady: () => true }, undefined, {
      secret: "mcp-secret", resource: "https://dipole.local/api/v1/agent/mcp", handler: { fetch }
    });
    expect((await server.inject({ method: "POST", url: "/internal/v1/agent/tasks/TASK-1/runs/RUN-1/mcp", payload: {} })).statusCode).toBe(401);
    expect((await server.inject({
      method: "POST", url: "/internal/v1/agent/tasks/TASK-1/runs/RUN-1/mcp",
      headers: { ...headers, "x-dipole-service-token": "wrong" }, payload: {}
    })).statusCode).toBe(401);
    expect((await server.inject({
      method: "POST", url: "/internal/v1/agent/tasks/TASK-1/runs/RUN-1/mcp",
      headers: { ...headers, "x-dipole-oauth-scope": "dipole.agent.mcp.write" }, payload: {}
    })).statusCode).toBe(401);
    expect(fetch).not.toHaveBeenCalled();
    for (const method of ["GET", "DELETE"] as const) {
      expect((await server.inject({
        method, url: "/internal/v1/agent/tasks/TASK-1/runs/RUN-1/mcp", headers
      })).statusCode).toBe(200);
    }
    expect(fetch).toHaveBeenCalledTimes(2);
    await server.close();
  });

  it("binds Task and Run to AuthInfo without exposing the service secret", async () => {
    const fetch = vi.fn(async (request: Request, options?: { authInfo?: unknown; parsedBody?: unknown }) => {
      expect(request.headers.get("x-dipole-service-token")).toBeNull();
      expect(request.headers.get("content-length")).toBeNull();
      expect(options?.parsedBody).toEqual({ jsonrpc: "2.0", id: 1, method: "tools/list", principalUserId: "U999" });
      expect(options?.authInfo).toEqual({
        token: "gateway-authenticated", clientId: "U100", scopes: ["dipole.agent.mcp.read"],
        extra: {
          resource: "https://dipole.local/api/v1/agent/mcp",
          taskId: "TASK-1", runId: "RUN-1", requestId: "R1", traceId: "T1"
        }
      });
      return new Response("event: message\ndata: accepted\n\n", {
        status: 202, headers: { "content-type": "text/event-stream", "mcp-session-id": "S1" }
      });
    });
    const server = buildServer({ isReady: () => true }, undefined, {
      secret: "mcp-secret", resource: "https://dipole.local/api/v1/agent/mcp", handler: { fetch }
    });
    const response = await server.inject({
      method: "POST", url: "/internal/v1/agent/tasks/TASK-1/runs/RUN-1/mcp", headers,
      payload: { jsonrpc: "2.0", id: 1, method: "tools/list", principalUserId: "U999" }
    });
    expect(response.statusCode).toBe(202);
    expect(response.headers["content-type"]).toContain("text/event-stream");
    expect(response.headers["mcp-session-id"]).toBe("S1");
    expect(response.body).toContain("data: accepted");
    expect(fetch).toHaveBeenCalledOnce();
    await server.close();
  });
});
