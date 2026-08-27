import { describe, expect, it } from "vitest";

import { createAgentTaskState, transitionAgentTask } from "./agent-task-state.js";

describe("Agent Task state", () => {
  it("moves through approval and completion with monotonic revisions", () => {
    const created = createAgentTaskState("task-1");
    const running = transitionAgentTask(created, { type: "start" });
    const waiting = transitionAgentTask(running, {
      type: "request_approval", requestId: "approval-1", approvalId: "APR-1", summary: "send one message"
    });
    const resumed = transitionAgentTask(waiting, {
      type: "resolve_approval", requestId: "approval-1", decision: "approved"
    });
    const completed = transitionAgentTask(resumed, { type: "complete", output: { artifactId: "A1" } });

    expect(created).toEqual({ taskId: "task-1", status: "created", revision: 0 });
    expect(waiting).toMatchObject({
      status: "waiting_approval", revision: 2,
      pending: { kind: "approval", requestId: "approval-1", approvalId: "APR-1", summary: "send one message" }
    });
    expect(resumed).toMatchObject({ status: "running", revision: 3 });
    expect(resumed).not.toHaveProperty("pending");
    expect(completed).toMatchObject({ status: "completed", revision: 4, output: { artifactId: "A1" } });
  });

  it("requires the exact pending input request before resuming", () => {
    const waiting = transitionAgentTask(
      transitionAgentTask(createAgentTaskState("task-2"), { type: "start" }),
      { type: "request_input", requestId: "input-1", prompt: "Choose a group" }
    );

    expect(() => transitionAgentTask(waiting, {
      type: "provide_input", requestId: "other", value: { groupId: "G1" }
    })).toThrow(/input-1/);
    expect(transitionAgentTask(waiting, {
      type: "provide_input", requestId: "input-1", value: { groupId: "G1" }
    })).toMatchObject({ status: "running", revision: 3, resume: { kind: "input", value: { groupId: "G1" } } });
  });

  it("turns a denied approval into a terminal cancellation", () => {
    const waiting = transitionAgentTask(
      transitionAgentTask(createAgentTaskState("task-3"), { type: "start" }),
      { type: "request_approval", requestId: "approval-3", approvalId: "APR-3", summary: "delete artifact" }
    );
    const cancelled = transitionAgentTask(waiting, {
      type: "resolve_approval", requestId: "approval-3", decision: "denied"
    });

    expect(cancelled).toMatchObject({
      status: "cancelled", revision: 3,
      cancellation: { reason: "approval_denied", requestId: "approval-3" }
    });
    expect(() => transitionAgentTask(cancelled, { type: "start" })).toThrow(/terminal/);
  });

  it("rejects invalid transitions without mutating the original state", () => {
    const created = createAgentTaskState("task-4");

    expect(() => transitionAgentTask(created, { type: "complete", output: null })).toThrow(/complete.*created/);
    expect(created).toEqual({ taskId: "task-4", status: "created", revision: 0 });
  });
});
