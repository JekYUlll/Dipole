import { describe, expect, it } from "vitest";

import { createAgentWorkflowRepairExecutionPlan } from "./agent-workflow-repair-execution-plan.js";

const current = { workflowId: "dipole-agent-task/TASK-1", workflowRunId: "WR-1", status: "running", revision: 2 } as const;
const target = { workflowId: "dipole-agent-task/TASK-1", workflowRunId: "WR-1", status: "completed", revision: 3 } as const;

function input() {
  return {
    proposalId: `repair:${"a".repeat(64)}`, proposalStatus: "approved" as const, proposalEvidenceSha256: "b".repeat(64),
    proposerId: "U-PROPOSER-1", executorId: "U-EXEC-1", executorGrantVersion: 7, changeTicketRef: "INC-2048",
    approverIds: ["U-APPROVER-1", "U-APPROVER-2"] as [string, string], expectedCurrentProjection: current,
    targetProjection: target, rollbackProjection: current,
    capturedAt: "2026-08-28T00:00:00.000Z", expiresAt: "2026-08-28T00:15:00.000Z"
  };
}

describe("Agent Workflow repair execution plan", () => {
  it("creates a deterministic dry-run plan with CAS evidence", () => {
    const first = createAgentWorkflowRepairExecutionPlan(input(), new Date("2026-08-28T00:05:00.000Z"));
    const second = createAgentWorkflowRepairExecutionPlan(input(), new Date("2026-08-28T00:05:00.000Z"));
    expect(first).toEqual(second);
    expect(first.mode).toBe("dry_run");
    expect(first.planId).toMatch(/^repair-plan:[a-f0-9]{64}$/u);
    expect(first.expectedCurrentSha256).toMatch(/^[a-f0-9]{64}$/u);
    expect(first.targetSha256).toMatch(/^[a-f0-9]{64}$/u);
    expect(first.rollbackSha256).toBe(first.expectedCurrentSha256);
    expect(first).not.toHaveProperty("apply");
  });

  it("rejects replay outside the window, identity reuse, and rollback drift", () => {
    expect(() => createAgentWorkflowRepairExecutionPlan(input(), new Date("2026-08-28T00:16:00.000Z"))).toThrow(/active window/);
    expect(() => createAgentWorkflowRepairExecutionPlan({ ...input(), executorId: "U-APPROVER-1" }, new Date("2026-08-28T00:05:00.000Z"))).toThrow(/identities/);
    expect(() => createAgentWorkflowRepairExecutionPlan({ ...input(), rollbackProjection: target }, new Date("2026-08-28T00:05:00.000Z"))).toThrow(/rollback/);
  });

  it("requires an explicit null rollback for a missing projection", () => {
    const plan = createAgentWorkflowRepairExecutionPlan({ ...input(), expectedCurrentProjection: null, rollbackProjection: null }, new Date("2026-08-28T00:05:00.000Z"));
    expect(plan.expectedCurrentSha256).toBeNull();
    expect(plan.rollbackSha256).toBeNull();
  });

  it("rejects a target projection from another Workflow Run", () => {
    expect(() => createAgentWorkflowRepairExecutionPlan({
      ...input(), targetProjection: { ...target, workflowRunId: "WR-OTHER" }
    }, new Date("2026-08-28T00:05:00.000Z"))).toThrow(/same Workflow Run/);
  });
});
