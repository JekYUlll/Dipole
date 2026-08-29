import { describe, expect, it, vi } from "vitest";

import { AgentTaskProjectionReconciler } from "./agent-task-projection-reconciler.js";

describe("AgentTaskProjectionReconciler", () => {
  it("classifies every projection outcome and emits bounded deterministic evidence", async () => {
    const source = {
      list: vi.fn()
        .mockResolvedValueOnce({
          tasks: [
            task("T-MATCH", "running", 2),
            task("T-MISSING"),
            task("T-STALE", "running", 1),
            task("T-AHEAD", "running", 4)
          ],
          nextCursor: "T-AHEAD"
        })
        .mockResolvedValueOnce({
          tasks: [task("T-CONFLICT", "waiting_input", 3), task("T-UNAVAILABLE", "running", 1)],
          nextCursor: ""
        })
    };
    const workflows = {
      inspect: vi.fn(async (taskId: string) => {
        if (taskId === "T-UNAVAILABLE") throw new Error("Temporal unavailable");
        const states = {
          "T-MATCH": state("T-MATCH", "running", 2),
          "T-MISSING": state("T-MISSING", "created", 0),
          "T-STALE": state("T-STALE", "running", 2),
          "T-AHEAD": state("T-AHEAD", "running", 2),
          "T-CONFLICT": state("T-CONFLICT", "waiting_approval", 3)
        } as const;
        return {
          workflowId: `dipole-agent-task/${taskId}`,
          workflowRunId: `WR-${taskId}`,
          state: states[taskId as keyof typeof states]
        };
      })
    };

    const report = await new AgentTaskProjectionReconciler(source, workflows).run({ pageSize: 4, maxExamples: 3 });

    expect(source.list).toHaveBeenNthCalledWith(1, "", 4);
    expect(source.list).toHaveBeenNthCalledWith(2, "T-AHEAD", 4);
    expect(report).toEqual({
      schemaVersion: "dipole.agent.projection-reconcile.v1",
      consistent: false,
      scanned: 6,
      outcomes: { match: 1, missing: 1, stale: 1, ahead: 1, conflict: 1, unavailable: 1 },
      examples: [
        expect.objectContaining({ taskId: "T-MISSING", outcome: "missing" }),
        expect.objectContaining({ taskId: "T-STALE", outcome: "stale", temporalRevision: 2 }),
        expect.objectContaining({ taskId: "T-AHEAD", outcome: "ahead", projectedRevision: 4 })
      ]
    });
  });

  it("fails closed on cursor loops and conflicting Workflow bindings", async () => {
    const looping = { list: vi.fn(async () => ({ tasks: [], nextCursor: "same" })) };
    const workflows = { inspect: vi.fn() };
    await expect(new AgentTaskProjectionReconciler(looping, workflows).run({ pageSize: 10, maxExamples: 10 }))
      .rejects.toThrow(/cursor did not advance/);

    const source = { list: vi.fn(async () => ({ tasks: [task("T1", "running", 1)], nextCursor: "" })) };
    const drift = { inspect: vi.fn(async () => ({ workflowId: "wrong", workflowRunId: "WR-T1", state: state("T1", "running", 1) })) };
    const report = await new AgentTaskProjectionReconciler(source, drift).run({ pageSize: 10, maxExamples: 10 });
    expect(report.outcomes.conflict).toBe(1);
    expect(report.examples[0]).toMatchObject({ taskId: "T1", outcome: "conflict", reason: "workflow_binding" });
  });
});

function task(taskId: string, status?: string, revision?: number) {
  return {
    taskId,
    ...(status === undefined ? {} : {
      workflow: {
        workflowId: `dipole-agent-task/${taskId}`,
        workflowRunId: `WR-${taskId}`,
        workflowStatus: status,
        workflowRevision: revision!
      }
    })
  };
}

function state(taskId: string, status: "created" | "running" | "waiting_input" | "waiting_approval", revision: number) {
  return { taskId, status, revision };
}
