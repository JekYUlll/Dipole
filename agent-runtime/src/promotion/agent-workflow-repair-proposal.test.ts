import { describe, expect, it } from "vitest";

import { createAgentWorkflowRepairProposal } from "./agent-workflow-repair-proposal.js";

describe("Agent Workflow repair proposal", () => {
  it("creates deterministic operator-bound audit evidence without applying a repair", () => {
    const input = {
      taskId: "TASK-1", outcome: "stale" as const, operatorId: "U-OPS-1", ticketRef: "INC-2048",
      reason: "verified Temporal state after Worker recovery",
      projected: { workflowId: "dipole-agent-task/TASK-1", workflowRunId: "WR-1", status: "running", revision: 2 },
      temporal: { workflowId: "dipole-agent-task/TASK-1", workflowRunId: "WR-1", status: "completed", revision: 3 },
      proposedAt: "2026-08-28T00:00:00.000Z", expiresAt: "2026-08-28T01:00:00.000Z"
    };
    const first = createAgentWorkflowRepairProposal(input);
    const second = createAgentWorkflowRepairProposal(input);
    expect(first).toEqual(second);
    expect(first).toMatchObject({
      schemaVersion: "dipole.agent.workflow-repair-proposal.v1", status: "proposed",
      action: "reproject_from_temporal", taskId: "TASK-1", operatorId: "U-OPS-1", ticketRef: "INC-2048"
    });
    expect(first.proposalId).toMatch(/^repair:[a-f0-9]{64}$/);
    expect(first.evidenceSha256).toMatch(/^[a-f0-9]{64}$/);
    expect(first).not.toHaveProperty("apply");
  });

  it("rejects unavailable evidence, model-controlled identity, and long-lived proposals", () => {
    const base = {
      taskId: "TASK-1", outcome: "unavailable" as const, operatorId: "", ticketRef: "",
      reason: "retry later", proposedAt: "2026-08-28T00:00:00.000Z", expiresAt: "2026-08-28T02:00:00.000Z"
    };
    expect(() => createAgentWorkflowRepairProposal(base)).toThrow(/repairable discrepancy/);
    expect(() => createAgentWorkflowRepairProposal({
      ...base, outcome: "stale", operatorId: "U1", ticketRef: "INC-1",
      temporal: { workflowId: "dipole-agent-task/TASK-1", workflowRunId: "WR-1", status: "running", revision: 1 }
    }))
      .toThrow(/one hour/);
  });
});
