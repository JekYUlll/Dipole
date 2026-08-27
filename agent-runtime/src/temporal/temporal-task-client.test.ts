import { describe, expect, it, vi } from "vitest";

import { agentTaskWorkflowId, TemporalShadowTaskDispatcher, TemporalTaskClient, TemporalTaskControlClient } from "./temporal-task-client.js";

describe("Temporal Task client", () => {
  it("derives one stable Workflow ID from the persistent Task ID", () => {
    expect(agentTaskWorkflowId("01K5TASKABC")).toBe("dipole-agent-task/01K5TASKABC");
    expect(agentTaskWorkflowId("01K5TASKABC")).toBe(agentTaskWorkflowId("01K5TASKABC"));
    expect(() => agentTaskWorkflowId("  ")).toThrow(/Task ID/);
  });

  it("reuses a running Workflow and rejects a new run after terminal completion", async () => {
    const start = vi.fn(async () => ({ workflowId: "dipole-agent-task/task-1", runId: "run-1" }));
    const client = new TemporalTaskClient({ start }, "dipole-agent-task-v1");

    const handle = await client.start({ taskId: "task-1", goal: "summarize G1" });

    expect(handle).toEqual({ workflowId: "dipole-agent-task/task-1", runId: "run-1" });
    expect(start).toHaveBeenCalledWith("agentTaskWorkflow", {
      taskQueue: "dipole-agent-task-v1",
      workflowId: "dipole-agent-task/task-1",
      workflowIdConflictPolicy: "USE_EXISTING",
      workflowIdReusePolicy: "REJECT_DUPLICATE",
      args: [{ taskId: "task-1", goal: "summarize G1" }]
    });
  });
});

describe("TemporalShadowTaskDispatcher", () => {
  it("starts a stable Workflow with trusted event admission", async () => {
    const start = vi.fn(async () => ({ workflowId: "dipole-agent-task/task-1" }));
    const dispatcher = new TemporalShadowTaskDispatcher({ start });
    await dispatcher.dispatch({
      eventId: "E1", eventType: "message.direct.created", aggregateId: "M1",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: { content: "hello" }
    }, {
      tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", requestId: "R1", traceId: "T1"
    }, "task-1");
    expect(start).toHaveBeenCalledWith({
      taskId: "task-1", goal: "observe message.direct.created for M1",
      shadowEvent: expect.objectContaining({ eventId: "E1", aggregateId: "M1" }),
      admission: {
        tenantId: "dipole", principalUserId: "U100", agentId: "UAI",
        triggerType: "message.direct.created", triggerRef: "M1", eventId: "E1", requestId: "R1", traceId: "T1"
      }
    });
  });
});

describe("TemporalTaskControlClient", () => {
  it("queries and Signals the stable Task Workflow", async () => {
    const query = vi.fn(async () => ({ taskId: "TASK-1", status: "running", revision: 1 }));
    const signal = vi.fn(async () => undefined);
    const getHandle = vi.fn(() => ({ query, signal }));
    const controls = new TemporalTaskControlClient({ getHandle });

    await expect(controls.query("TASK-1")).resolves.toMatchObject({ taskId: "TASK-1", status: "running" });
    await controls.cancel("TASK-1", "user_cancelled");
    await controls.resolveApproval("TASK-1", {
      requestId: "REQ-1", approvalId: "APR-1", decision: "approved", actorUserId: "U100"
    });

    expect(getHandle).toHaveBeenCalledWith("dipole-agent-task/TASK-1");
    expect(query).toHaveBeenCalledWith("taskState");
    expect(signal).toHaveBeenCalledWith("cancelTask", { reason: "user_cancelled" });
    expect(signal).toHaveBeenCalledWith("resolveTaskApproval", {
      requestId: "REQ-1", approvalId: "APR-1", decision: "approved", actorUserId: "U100"
    });
  });
});
