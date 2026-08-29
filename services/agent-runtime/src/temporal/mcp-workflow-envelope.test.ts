import { describe, expect, it, vi } from "vitest";

import {
  TemporalMcpWorkflowExecutionCatalog,
  createTemporalMcpBeginActivityInput,
  validateTemporalMcpWorkflowExecution
} from "./mcp-workflow-envelope.js";
import { TemporalMcpTaskClient } from "./temporal-task-client.js";

const calendarBinding = {
  routeId: "calendar-event-read",
  routeVersion: 3,
  routeManifestSha256: "a".repeat(64)
};
const issueBinding = {
  routeId: "github-issue-read",
  routeVersion: 2,
  routeManifestSha256: "b".repeat(64)
};

describe("Temporal MCP Workflow execution envelope", () => {
  it("derives version and manifest authority from the host catalog", () => {
    const catalog = new TemporalMcpWorkflowExecutionCatalog([calendarBinding, issueBinding]);

    const execution = catalog.create("calendar-event-read", { eventId: "EV-1", calendarId: "CAL-1" });

    expect(execution).toEqual({
      kind: "external_mcp_v1",
      ...calendarBinding,
      arguments: { calendarId: "CAL-1", eventId: "EV-1" }
    });
    expect(Object.keys(execution)).not.toContain("profileId");
    expect(Object.keys(execution)).not.toContain("toolName");
  });

  it("rejects unknown or duplicate routes and invalid business arguments", () => {
    const catalog = new TemporalMcpWorkflowExecutionCatalog([calendarBinding]);
    expect(() => catalog.create("missing", {})).toThrow(/route is unavailable/i);
    expect(() => new TemporalMcpWorkflowExecutionCatalog([calendarBinding, calendarBinding]))
      .toThrow(/route ID is duplicated/i);
    expect(() => catalog.create(calendarBinding.routeId, { value: BigInt(1) }))
      .toThrow(/JSON serializable/i);
    expect(() => catalog.create(calendarBinding.routeId, { value: "x".repeat(17 * 1024) }))
      .toThrow(/16 KiB/i);
  });

  it("validates persisted history and derives Activity identity from admission", () => {
    const execution = new TemporalMcpWorkflowExecutionCatalog([calendarBinding])
      .create(calendarBinding.routeId, { calendarId: "CAL-1", eventId: "EV-1" });

    expect(createTemporalMcpBeginActivityInput(execution, {
      taskId: "TASK-1",
      runId: "RUN-1",
      principalUserId: "U100",
      requestId: "REQ-1",
      traceId: "TRACE-1"
    })).toEqual({
      kind: "begin",
      ...calendarBinding,
      taskId: "TASK-1",
      runId: "RUN-1",
      principalUserId: "U100",
      arguments: { calendarId: "CAL-1", eventId: "EV-1" },
      requestId: "REQ-1",
      traceId: "TRACE-1"
    });
    expect(() => validateTemporalMcpWorkflowExecution({
      ...execution,
      profileId: "forged-profile"
    })).toThrow(/execution envelope is invalid/i);
  });

  it("starts the generic Workflow through a dedicated host-derived MCP client", async () => {
    const start = vi.fn(async () => ({
      workflowId: "dipole-agent-task/TASK-1",
      firstExecutionRunId: "TEMPORAL-RUN-1"
    }));
    const client = new TemporalMcpTaskClient(
      { start },
      "dipole-agent-task-v1",
      new TemporalMcpWorkflowExecutionCatalog([calendarBinding])
    );

    await expect(client.start({
      taskId: "TASK-1",
      goal: "read one calendar event",
      routeId: calendarBinding.routeId,
      arguments: { calendarId: "CAL-1", eventId: "EV-1" },
      admission: {
        tenantId: "dipole",
        principalUserId: "U100",
        agentId: "UAI",
        triggerType: "user_request",
        triggerRef: "CONV-1",
        eventId: "EVENT-1",
        requestId: "REQ-1",
        traceId: "TRACE-1"
      }
    })).resolves.toEqual({
      workflowId: "dipole-agent-task/TASK-1",
      runId: "TEMPORAL-RUN-1"
    });

    expect(start).toHaveBeenCalledWith("agentTaskWorkflow", {
      taskQueue: "dipole-agent-task-v1",
      workflowId: "dipole-agent-task/TASK-1",
      workflowIdConflictPolicy: "USE_EXISTING",
      workflowIdReusePolicy: "REJECT_DUPLICATE",
      args: [{
        taskId: "TASK-1",
        goal: "read one calendar event",
        admission: expect.objectContaining({ principalUserId: "U100", requestId: "REQ-1" }),
        execution: {
          kind: "external_mcp_v1",
          ...calendarBinding,
          arguments: { calendarId: "CAL-1", eventId: "EV-1" }
        }
      }]
    });
  });
});
