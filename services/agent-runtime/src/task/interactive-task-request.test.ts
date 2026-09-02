import { describe, expect, it, vi } from "vitest";

import { createInteractiveTaskRequest, InteractiveTaskStartService } from "./interactive-task-request.js";

const trusted = { tenantId: "dipole", principalUserId: "U100", agentId: "UAI", requestId: "REQ-1", traceId: "TRACE-1" };

describe("interactive Agent Task request", () => {
  it("creates a deterministic Task and a bounded Runtime-owned event", () => {
    const first = createInteractiveTaskRequest({ clientRequestId: "client-1", goal: "Summarize my unread work." }, trusted, new Date("2026-08-31T00:00:00.000Z"));
    const replay = createInteractiveTaskRequest({ clientRequestId: "client-1", goal: "Summarize my unread work." }, trusted, new Date("2026-08-31T00:00:01.000Z"));

    expect(replay.taskId).toBe(first.taskId);
    expect(first).toMatchObject({
      identity: { tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", requestId: "REQ-1", traceId: "TRACE-1" },
      event: { eventType: "agent.interactive.requested", payload: { content: "Summarize my unread work.", request_kind: "interactive" } }
    });
    expect(first.event.aggregateId).toMatch(/^interactive:[a-f0-9]{48}$/);
    expect(first.event.eventId).toBe(first.event.aggregateId);
  });

  it("scopes the client idempotency key to the authenticated principal", () => {
    const first = createInteractiveTaskRequest({ clientRequestId: "client-1", goal: "Summarize my unread work." }, trusted);
    const second = createInteractiveTaskRequest({ clientRequestId: "client-1", goal: "Summarize my unread work." }, { ...trusted, principalUserId: "U200" });

    expect(second.taskId).not.toBe(first.taskId);
    expect(second.event.eventId).not.toBe(first.event.eventId);
  });

  it("rejects identity and client input that could make the request ambiguous", () => {
    expect(() => createInteractiveTaskRequest({ clientRequestId: "", goal: "x" }, trusted)).toThrow();
    expect(() => createInteractiveTaskRequest({ clientRequestId: "client-1", goal: "x", principalUserId: "U999" }, trusted)).toThrow();
    expect(() => createInteractiveTaskRequest({ clientRequestId: "client-1", goal: "x" }, { ...trusted, agentId: " " })).toThrow(/Agent ID/i);
  });

  it("dispatches only a request bound to the trusted Gateway identity", async () => {
    const dispatch = vi.fn(async () => undefined);
    const service = new InteractiveTaskStartService({ tenantId: "dipole", agentId: "UAI" }, { dispatch });

    const result = await service.start({
      principalUserId: "U100", requestId: "REQ-1", body: { clientRequestId: "client-1", goal: "Summarize my unread work." }
    });

    expect(result).toEqual(expect.objectContaining({ status: "accepted" }));
    expect(dispatch).toHaveBeenCalledWith(expect.objectContaining({ eventType: "agent.interactive.requested" }), expect.objectContaining({
      principalUuid: "U100", agentUuid: "UAI"
    }), result.taskId);
    await expect(service.start({ principalUserId: "U100", body: { clientRequestId: "", goal: "invalid" } }))
      .rejects.toMatchObject({ code: "invalid_argument" });
  });
});
