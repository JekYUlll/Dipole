import { z } from "zod";
import { describe, expect, it, vi } from "vitest";

import type { ExecutionContext } from "../runtime/execution-context.js";
import {
  ExternalMcpCapabilityRouteRegistry,
  TrustedMcpInvocationProducer
} from "./mcp-invocation-producer.js";

const context: ExecutionContext = {
  tenantId: "dipole",
  principalUuid: "U100",
  agentUuid: "UAI",
  taskId: "TASK-1",
  runId: "RUN-1",
  mode: "shadow",
  permissions: ["calendar.read"],
  resourceScopes: [{ resourceType: "calendar", resourceId: "CAL-1", actions: ["read"] }],
  approvedCapabilities: [],
  requestId: "REQ-1",
  traceId: "TRACE-1"
};

function registry(): ExternalMcpCapabilityRouteRegistry {
  const routes = new ExternalMcpCapabilityRouteRegistry();
  routes.register({
    descriptor: { id: "calendar.event.read", risk: "read", requiredPermission: "calendar.read" },
    inputSchema: z.object({ calendarId: z.string(), eventId: z.string(), details: z.unknown().optional() }).strict(),
    profileId: "calendar-prod",
    serverId: "calendar.example",
    toolName: "calendar.read_event",
    egressPolicy: { allowedArgumentNames: ["calendarId", "eventId", "details"], maximumBytes: 1024 },
    resolveResource: input => ({ resourceType: "calendar", resourceId: input.calendarId, action: "read" })
  });
  return routes;
}

describe("trusted MCP Invocation producer", () => {
  it("derives a stable host-owned ID and resolves all external authority from the route", async () => {
    const beginMcpToolCommand = vi.fn(async (input) => ({ invocationId: input.invocationId, status: "running" as const }));
    const producer = new TrustedMcpInvocationProducer(registry(), { beginMcpToolCommand });
    const input = {
      workflowStep: 3,
      ordinal: 1,
      capabilityId: "calendar.event.read",
      arguments: { calendarId: "CAL-1", eventId: "EV-7" }
    };

    const first = await producer.produce(input, context);
    const replay = await producer.produce(input, context);
    const argumentDrift = await producer.produce({
      ...input, arguments: { calendarId: "CAL-1", eventId: "EV-8" }
    }, context);
    const nextOrdinal = await producer.produce({ ...input, ordinal: 2 }, context);

    expect(first).toEqual({
      invocationId: "17009e948b773e9f036a8d54c4bf8eb90b25d80e1ae7db6a9f36adfa75f58158",
      status: "running",
      taskId: "TASK-1",
      runId: "RUN-1"
    });
    expect(replay.invocationId).toBe(first.invocationId);
    expect(argumentDrift.invocationId).toBe(first.invocationId);
    expect(nextOrdinal.invocationId).not.toBe(first.invocationId);
    expect(beginMcpToolCommand).toHaveBeenCalledWith({
      invocationId: first.invocationId,
      taskId: "TASK-1",
      runId: "RUN-1",
      toolName: "calendar.read_event",
      capabilityId: "calendar.event.read",
      argumentsSha256: "d7a46a5ed89ec301a307a244eefca69c981561f4f458ef593c8a88984d2f00d1",
      profileId: "calendar-prod",
      serverId: "calendar.example",
      argumentsJson: `{"calendarId":"CAL-1","eventId":"EV-7"}`,
      requestId: "REQ-1",
      traceId: "TRACE-1"
    });
  });

  it("propagates terminal begin evidence for receipt-only Worker recovery", async () => {
    const beginMcpToolCommand = vi.fn(async (input) => ({ invocationId: input.invocationId, status: "completed" as const }));
    const producer = new TrustedMcpInvocationProducer(registry(), { beginMcpToolCommand });

    await expect(producer.produce({
      workflowStep: 3, ordinal: 1, capabilityId: "calendar.event.read",
      arguments: { calendarId: "CAL-1", eventId: "EV-7" }
    }, context)).resolves.toMatchObject({ status: "completed" });
  });

  it("derives the Worker egress policy from the registered Capability route", () => {
    const routes = registry();
    const policies = routes.workerEgressPolicies("calendar.event.read");

    expect(policies).toEqual({
      "calendar-prod": {
        "calendar.read_event": {
          allowedArgumentNames: ["calendarId", "eventId", "details"], maximumBytes: 1024
        }
      }
    });
    expect(() => routes.workerEgressPolicies("calendar.event.write")).toThrow(/unavailable/i);
  });

  it("rejects caller-supplied authority before route or Core access", async () => {
    const beginMcpToolCommand = vi.fn();
    const producer = new TrustedMcpInvocationProducer(registry(), { beginMcpToolCommand });

    await expect(producer.produce({
      workflowStep: 3,
      ordinal: 1,
      capabilityId: "calendar.event.read",
      arguments: { calendarId: "CAL-1", eventId: "EV-7" },
      profileId: "forged",
      principalUserId: "attacker"
    }, context)).rejects.toThrow();
    expect(beginMcpToolCommand).not.toHaveBeenCalled();
  });

  it.each([
    ["resource scope", { ...context, resourceScopes: [{ resourceType: "calendar", resourceId: "CAL-2", actions: ["read"] }] }, { calendarId: "CAL-1", eventId: "EV-7" }],
    ["undeclared argument", context, { calendarId: "CAL-1", eventId: "EV-7", extra: true }],
    ["sensitive field", context, { calendarId: "CAL-1", eventId: "EV-7", details: { token: "hidden" } }]
  ])("blocks %s before Core begin", async (_name, executionContext, arguments_) => {
    const beginMcpToolCommand = vi.fn();
    const producer = new TrustedMcpInvocationProducer(registry(), { beginMcpToolCommand });

    await expect(producer.produce({
      workflowStep: 3, ordinal: 1, capabilityId: "calendar.event.read", arguments: arguments_
    }, executionContext as ExecutionContext)).rejects.toThrow();
    expect(beginMcpToolCommand).not.toHaveBeenCalled();
  });
});
