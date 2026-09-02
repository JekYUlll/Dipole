import { describe, expect, it, vi } from "vitest";

import { agentTaskId, type AgentEvent, type AgentIdentity } from "../events/shadow-processor.js";
import { TemporalMcpShadowTaskDispatcher } from "./mcp-shadow-task-dispatcher.js";
import { TemporalMcpWorkflowExecutionCatalog } from "./mcp-workflow-envelope.js";
import { TemporalMcpTaskClient } from "./temporal-task-client.js";

const binding = {
  routeId: "github-issue-read",
  routeVersion: 2,
  routeManifestSha256: "a".repeat(64)
};
const event: AgentEvent = {
  eventId: "EVENT-1",
  eventType: "message.direct.created",
  aggregateId: "MESSAGE-1",
  occurredAt: "2026-08-28T08:00:00.000Z",
  payload: { content: "inspect issue 42" },
  subscriptionId: "SUB-1"
};
const identity: AgentIdentity = {
  tenantId: "dipole",
  principalUuid: "U100",
  agentUuid: "UAI",
  requestId: "REQ-1",
  traceId: "TRACE-1"
};
const taskId = agentTaskId({
  tenantId: identity.tenantId,
  agentUuid: identity.agentUuid,
  triggerType: event.eventType,
  triggerRef: event.aggregateId,
  ...(event.subscriptionId === undefined ? {} : { subscriptionId: event.subscriptionId })
});

describe("TemporalMcpShadowTaskDispatcher", () => {
  it("derives admission and Workflow authority from trusted event inputs and the host route catalog", async () => {
    const start = vi.fn(async () => ({
      workflowId: `dipole-agent-task/${taskId}`,
      firstExecutionRunId: "TEMPORAL-RUN-1"
    }));
    const select = vi.fn(() => ({
      routeId: binding.routeId,
      arguments: { owner: "dipole", repo: "server", issue_number: 42 }
    }));
    const tasks = new TemporalMcpTaskClient(
      { start },
      "dipole-agent-task-v1",
      new TemporalMcpWorkflowExecutionCatalog([binding])
    );
    const dispatcher = new TemporalMcpShadowTaskDispatcher(tasks, { select });

    await dispatcher.dispatch(event, identity, taskId);

    expect(select).toHaveBeenCalledWith(event, identity);
    expect(start).toHaveBeenCalledWith("agentTaskWorkflow", expect.objectContaining({
      taskQueue: "dipole-agent-task-v1",
      workflowId: `dipole-agent-task/${taskId}`,
      args: [{
        taskId,
        goal: "execute external MCP route github-issue-read",
        admission: {
          tenantId: "dipole",
          principalUserId: "U100",
          agentId: "UAI",
          triggerType: "message.direct.created",
          triggerRef: "MESSAGE-1",
          eventId: "EVENT-1",
          subscriptionId: "SUB-1",
          requestId: "REQ-1",
          traceId: "TRACE-1"
        },
        execution: {
          kind: "external_mcp_v1",
          ...binding,
          arguments: { issue_number: 42, owner: "dipole", repo: "server" }
        }
      }]
    }));
  });

  it("rejects a mismatched Task ID before route selection or Workflow start", async () => {
    const start = vi.fn();
    const select = vi.fn(() => ({ routeId: binding.routeId, arguments: {} }));
    const dispatcher = new TemporalMcpShadowTaskDispatcher({ start }, { select });

    await expect(dispatcher.dispatch(event, identity, "task:forged"))
      .rejects.toThrow(/deterministic event binding/i);
    expect(select).not.toHaveBeenCalled();
    expect(start).not.toHaveBeenCalled();
  });

  it("rejects selector output that attempts to add admission authority", async () => {
    const start = vi.fn();
    const dispatcher = new TemporalMcpShadowTaskDispatcher({ start }, {
      select: vi.fn(() => ({
        routeId: binding.routeId,
        arguments: {},
        admission: { tenantId: "forged" }
      }))
    });

    await expect(dispatcher.dispatch(event, identity, taskId))
      .rejects.toThrow(/route selection is invalid/i);
    expect(start).not.toHaveBeenCalled();
  });

  it("freezes trusted route-selection inputs before building Workflow history", async () => {
    const start = vi.fn(async () => ({ workflowId: `dipole-agent-task/${taskId}` }));
    const select = vi.fn((selectedEvent: AgentEvent, selectedIdentity: AgentIdentity) => {
      expect(Object.isFrozen(selectedEvent)).toBe(true);
      expect(Object.isFrozen(selectedIdentity)).toBe(true);
      expect(() => {
        (selectedIdentity as { tenantId: string }).tenantId = "forged";
      }).toThrow();
      return { routeId: binding.routeId, arguments: {} };
    });
    const dispatcher = new TemporalMcpShadowTaskDispatcher({ start }, { select });

    await dispatcher.dispatch(event, identity, taskId);

    expect(start).toHaveBeenCalledWith(expect.objectContaining({
      admission: expect.objectContaining({ tenantId: "dipole", principalUserId: "U100" })
    }));
  });

  it("normalizes trusted identity before deriving admission", async () => {
    const start = vi.fn(async () => ({ workflowId: `dipole-agent-task/${taskId}` }));
    const select = vi.fn(() => ({ routeId: binding.routeId, arguments: {} }));
    const dispatcher = new TemporalMcpShadowTaskDispatcher({ start }, { select });

    await dispatcher.dispatch(event, {
      tenantId: " dipole ", principalUuid: " U100 ", agentUuid: " UAI ", requestId: " REQ-1 "
    }, taskId);

    expect(select).toHaveBeenCalledWith(event, {
      tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", requestId: "REQ-1"
    });
    expect(start).toHaveBeenCalledWith(expect.objectContaining({
      admission: expect.objectContaining({ tenantId: "dipole", principalUserId: "U100", agentId: "UAI" })
    }));
  });
});
