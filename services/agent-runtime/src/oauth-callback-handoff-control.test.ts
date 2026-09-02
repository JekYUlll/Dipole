import { describe, expect, it } from "vitest";

import {
  BoundedOAuthCallbackHandoffNotificationDeduplicator,
  buildServer,
  type OAuthCallbackHandoffControlAPI
} from "./server.js";

const path = "/internal/v1/agent/oauth/callback-handoffs";
const headers = {
  "x-dipole-caller-service": "dipole-gateway",
  "x-dipole-service-token": "control-secret",
  "x-request-id": "REQ-1",
  "x-trace-id": "TRACE-1"
};

describe("OAuth callback handoff control HTTP", () => {
  it("authenticates Gateway, preserves only correlation, and deduplicates repeated handoffs", async () => {
    const notifications: unknown[] = [];
    const server = buildServer({ isReady: () => true }, undefined, undefined, undefined, {
      secret: "control-secret",
      service: { async notifyHandoff(notification) { notifications.push(notification); } }
    });
    const payload = { handoff_id: "a".repeat(22) };
    expect((await server.inject({ method: "POST", url: path, headers, payload })).statusCode).toBe(202);
    expect((await server.inject({ method: "POST", url: path, headers, payload })).statusCode).toBe(202);
    expect(notifications).toEqual([{ handoffId: "a".repeat(22), requestId: "REQ-1", traceId: "TRACE-1" }]);
    await server.close();
  });

  it("rejects missing secrets, principal headers, and non-minimal bodies", async () => {
    const service: OAuthCallbackHandoffControlAPI = { async notifyHandoff() {} };
    const server = buildServer({ isReady: () => true }, undefined, undefined, undefined, { secret: "control-secret", service });
    expect((await server.inject({ method: "POST", url: path, payload: { handoff_id: "a".repeat(22) } })).statusCode).toBe(401);
    expect((await server.inject({ method: "POST", url: path, headers: { ...headers, "x-dipole-principal-user-id": "U100" }, payload: { handoff_id: "a".repeat(22) } })).statusCode).toBe(401);
    expect((await server.inject({ method: "POST", url: path, headers, payload: { handoff_id: "a".repeat(22), authorization_code: "forbidden" } })).statusCode).toBe(400);
    await server.close();
  });

  it("releases a failed notification so a later retry can be accepted", async () => {
    let failures = 1;
    const deduplicator = new BoundedOAuthCallbackHandoffNotificationDeduplicator(1);
    const server = buildServer({ isReady: () => true }, undefined, undefined, undefined, {
      secret: "control-secret", deduplicator,
      service: { async notifyHandoff() { if (failures-- > 0) throw new Error("unavailable"); } }
    });
    const request = { method: "POST" as const, url: path, headers, payload: { handoff_id: "a".repeat(22) } };
    expect((await server.inject(request)).statusCode).toBe(503);
    expect((await server.inject(request)).statusCode).toBe(202);
    await server.close();
  });
});
