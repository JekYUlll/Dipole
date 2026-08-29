import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { AgentTaskProjectionReconciler, type ProjectionReconcileOutcome } from "./agent-task-projection-reconciler.js";

interface EvalCase {
  id: string;
  projected_status?: string;
  projected_revision?: number;
  temporal_status?: "created" | "running" | "waiting_input" | "waiting_approval";
  temporal_revision?: number;
  temporal_error?: string;
  expected: ProjectionReconcileOutcome;
}

describe("Agent projection reconciliation Eval v1", () => {
  it("matches every versioned outcome fixture", async () => {
    const path = fileURLToPath(new URL("../../../../contracts/agent-evals/v1/projection-reconcile.json", import.meta.url));
    const suite = JSON.parse(await readFile(path, "utf8")) as { schema_version: string; cases: EvalCase[] };
    expect(suite.schema_version).toBe("dipole.agent.projection-reconcile.eval.v1");
    for (const evalCase of suite.cases) {
      const taskId = evalCase.id;
      const workflow = evalCase.projected_status === undefined ? undefined : {
        workflowId: `dipole-agent-task/${taskId}`, workflowRunId: `WR-${taskId}`,
        workflowStatus: evalCase.projected_status, workflowRevision: evalCase.projected_revision!
      };
      const report = await new AgentTaskProjectionReconciler({
        list: async () => ({ tasks: [{ taskId, ...(workflow === undefined ? {} : { workflow }) }], nextCursor: "" })
      }, {
        inspect: async () => {
          if (evalCase.temporal_error !== undefined) throw new Error(evalCase.temporal_error);
          return {
            workflowId: `dipole-agent-task/${taskId}`, workflowRunId: `WR-${taskId}`,
            state: { taskId, status: evalCase.temporal_status!, revision: evalCase.temporal_revision! }
          };
        }
      }).run({ pageSize: 10, maxExamples: 10 });
      expect(report.outcomes[evalCase.expected], evalCase.id).toBe(1);
    }
  });
});
