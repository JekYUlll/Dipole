import { describe, expect, it, vi } from "vitest";

import { AgentTaskControlError, AgentTaskControlService } from "./agent-task-control.js";

describe("AgentTaskControlService", () => {
  it("authorizes every query and returns the bound Workflow state", async () => {
    const authorizeTaskControl = vi.fn(async () => ({
      taskId: "TASK-1", taskStatus: "running",
      workflow: {
        taskId: "TASK-1", workflowId: "dipole-agent-task/TASK-1", workflowRunId: "temporal-run-1",
        workflowStatus: "running", workflowRevision: 2
      }
    }));
    const query = vi.fn(async () => ({ taskId: "TASK-1", status: "running" as const, revision: 2 }));
    const service = new AgentTaskControlService({ authorizeTaskControl }, { query, cancel: vi.fn(), resolveApproval: vi.fn(), provideInput: vi.fn() });

    await expect(service.getTask({ taskId: "TASK-1", principalUserId: "U100", requestId: "R1", traceId: "T1" })).resolves.toEqual({
      taskId: "TASK-1", status: "running", revision: 2, persistentStatus: "running",
      workflowProjection: { outcome: "match", status: "running", revision: 2 }
    });
    expect(authorizeTaskControl).toHaveBeenCalledWith("TASK-1", "U100", { requestId: "R1", traceId: "T1" });
  });

  it("reports stale Workflow projection evidence without repairing during a query", async () => {
    const service = new AgentTaskControlService({ authorizeTaskControl: vi.fn(async () => ({
      taskId: "TASK-1", taskStatus: "running",
      workflow: {
        taskId: "TASK-1", workflowId: "dipole-agent-task/TASK-1", workflowRunId: "temporal-run-1",
        workflowStatus: "running", workflowRevision: 1
      }
    })) }, {
      query: vi.fn(async () => ({ taskId: "TASK-1", status: "waiting_input" as const, revision: 2 })),
      cancel: vi.fn(), resolveApproval: vi.fn(), provideInput: vi.fn()
    });
    await expect(service.getTask({ taskId: "TASK-1", principalUserId: "U100" })).resolves.toMatchObject({
      workflowProjection: { outcome: "stale", status: "running", revision: 1 }
    });
  });

  it("binds approval Signals to the current pending request", async () => {
    const authorizeTaskControl = vi.fn(async () => ({ taskId: "TASK-1", taskStatus: "waiting_approval" }));
    const query = vi.fn(async () => ({
      taskId: "TASK-1", status: "waiting_approval" as const, revision: 3,
      pending: { kind: "approval" as const, requestId: "REQ-1", approvalId: "APR-1", summary: "send messages" }
    }));
    const resolveApproval = vi.fn(async () => undefined);
    const service = new AgentTaskControlService({ authorizeTaskControl }, { query, cancel: vi.fn(), resolveApproval, provideInput: vi.fn() });

    await service.resolveApproval({ taskId: "TASK-1", principalUserId: "U100", approvalId: "APR-1", decision: "approved" });
    expect(resolveApproval).toHaveBeenCalledWith("TASK-1", {
      requestId: "REQ-1", approvalId: "APR-1", decision: "approved", actorUserId: "U100"
    });
  });

  it("rejects stale approval IDs before sending a Signal", async () => {
    const resolveApproval = vi.fn();
    const service = new AgentTaskControlService(
      { authorizeTaskControl: vi.fn(async () => ({ taskId: "TASK-1", taskStatus: "waiting_approval" })) },
      { query: vi.fn(async () => ({
        taskId: "TASK-1", status: "waiting_approval" as const, revision: 3,
        pending: { kind: "approval" as const, requestId: "REQ-1", approvalId: "APR-CURRENT", summary: "send" }
      })), cancel: vi.fn(), resolveApproval, provideInput: vi.fn() }
    );

    await expect(service.resolveApproval({
      taskId: "TASK-1", principalUserId: "U100", approvalId: "APR-OLD", decision: "approved"
    })).rejects.toMatchObject({ code: "conflict" });
    expect(resolveApproval).not.toHaveBeenCalled();
  });

  it("authorizes and validates the exact pending input before sending a Signal", async () => {
    const provideInput = vi.fn(async () => undefined);
    const pending = {
      kind: "input" as const, requestId: "INPUT-1", prompt: "Choose scope",
      form: { schemaVersion: "dipole.agent.elicitation.v1" as const, fields: [
        { id: "scope", label: "Scope", type: "select" as const, required: true, options: ["today", "week"] }
      ] }
    };
    const service = new AgentTaskControlService(
      { authorizeTaskControl: vi.fn(async () => ({ taskId: "TASK-1", taskStatus: "running" })) },
      { query: vi.fn(async () => ({ taskId: "TASK-1", status: "waiting_input" as const, revision: 2, pending })), cancel: vi.fn(), resolveApproval: vi.fn(), provideInput }
    );

    await service.provideInput({ taskId: "TASK-1", principalUserId: "U100", requestId: "INPUT-1", value: { scope: "today" } });
    expect(provideInput).toHaveBeenCalledWith("TASK-1", { requestId: "INPUT-1", value: { scope: "today" } });
    await expect(service.provideInput({ taskId: "TASK-1", principalUserId: "U100", requestId: "INPUT-1", value: { scope: "month" } })).rejects.toMatchObject({ code: "invalid_argument" });
    await expect(service.provideInput({ taskId: "TASK-1", principalUserId: "U100", requestId: "INPUT-OLD", value: { scope: "today" } })).rejects.toMatchObject({ code: "conflict" });
  });

  it("authorizes cancellation and bounds its reason", async () => {
    const cancel = vi.fn(async () => undefined);
    const service = new AgentTaskControlService(
      { authorizeTaskControl: vi.fn(async () => ({ taskId: "TASK-1", taskStatus: "running" })) },
      { query: vi.fn(async () => ({ taskId: "TASK-1", status: "running" as const, revision: 1 })), cancel, resolveApproval: vi.fn(), provideInput: vi.fn() }
    );
    await service.cancelTask({ taskId: "TASK-1", principalUserId: "U100", reason: ` user_cancelled ${"x".repeat(300)}` });
    expect(cancel).toHaveBeenCalledWith("TASK-1", `user_cancelled ${"x".repeat(241)}`);
  });

  it("does not acknowledge cancellation for a terminal Workflow", async () => {
    const cancel = vi.fn();
    const service = new AgentTaskControlService(
      { authorizeTaskControl: vi.fn(async () => ({ taskId: "TASK-1", taskStatus: "completed" })) },
      { query: vi.fn(async () => ({ taskId: "TASK-1", status: "completed" as const, revision: 4 })), cancel, resolveApproval: vi.fn(), provideInput: vi.fn() }
    );
    await expect(service.cancelTask({ taskId: "TASK-1", principalUserId: "U100" })).rejects.toMatchObject({ code: "conflict" });
    expect(cancel).not.toHaveBeenCalled();
  });

  it("rejects conflicting Core and Workflow Task bindings", async () => {
    const service = new AgentTaskControlService(
      { authorizeTaskControl: vi.fn(async () => ({ taskId: "TASK-2", taskStatus: "running" })) },
      { query: vi.fn(), cancel: vi.fn(), resolveApproval: vi.fn(), provideInput: vi.fn() }
    );
    await expect(service.getTask({ taskId: "TASK-1", principalUserId: "U100" })).rejects.toBeInstanceOf(AgentTaskControlError);
  });
});
