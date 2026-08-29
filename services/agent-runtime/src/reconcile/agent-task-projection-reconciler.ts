import type { AgentTaskState } from "../task/agent-task-state.js";

export type ProjectionReconcileOutcome = "match" | "missing" | "stale" | "ahead" | "conflict" | "unavailable";

export interface AgentTaskProjectionSnapshot {
  taskId: string;
  workflow?: {
    workflowId: string;
    workflowRunId: string;
    workflowStatus: string;
    workflowRevision: number;
  };
}

export interface AgentTaskProjectionPageSource {
  list(afterTaskId: string, limit: number): Promise<{ tasks: readonly AgentTaskProjectionSnapshot[]; nextCursor: string }>;
}

export interface AgentTaskWorkflowInspector {
  inspect(taskId: string): Promise<{ workflowId: string; workflowRunId: string; state: AgentTaskState }>;
}

export interface AgentTaskProjectionReconcileExample {
  taskId: string;
  outcome: Exclude<ProjectionReconcileOutcome, "match">;
  reason?: string;
  projectedStatus?: string;
  projectedRevision?: number;
  temporalStatus?: string;
  temporalRevision?: number;
}

export interface AgentTaskProjectionReconcileReport {
  schemaVersion: "dipole.agent.projection-reconcile.v1";
  consistent: boolean;
  scanned: number;
  outcomes: Record<ProjectionReconcileOutcome, number>;
  examples: AgentTaskProjectionReconcileExample[];
}

export class AgentTaskProjectionReconciler {
  constructor(
    private readonly source: AgentTaskProjectionPageSource,
    private readonly workflows: AgentTaskWorkflowInspector
  ) {}

  async run(options: { pageSize: number; maxExamples: number }): Promise<AgentTaskProjectionReconcileReport> {
    const pageSize = boundedInteger(options.pageSize, 1, 1000, "page size");
    const maxExamples = boundedInteger(options.maxExamples, 0, 1000, "max examples");
    const outcomes: Record<ProjectionReconcileOutcome, number> = {
      match: 0, missing: 0, stale: 0, ahead: 0, conflict: 0, unavailable: 0
    };
    const examples: AgentTaskProjectionReconcileExample[] = [];
    let cursor = "";
    let scanned = 0;

    for (;;) {
      const page = await this.source.list(cursor, pageSize);
      for (const task of page.tasks) {
        const result = await this.reconcile(task);
        outcomes[result.outcome] += 1;
        scanned += 1;
        if (result.outcome !== "match" && examples.length < maxExamples) examples.push(result);
      }
      if (page.nextCursor === "") break;
      if (page.nextCursor <= cursor) throw new Error("Agent projection reconciliation cursor did not advance");
      cursor = page.nextCursor;
    }

    return {
      schemaVersion: "dipole.agent.projection-reconcile.v1",
      consistent: outcomes.match === scanned,
      scanned,
      outcomes,
      examples
    };
  }

  private async reconcile(task: AgentTaskProjectionSnapshot): Promise<AgentTaskProjectionReconcileExample | { taskId: string; outcome: "match" }> {
    let temporal: Awaited<ReturnType<AgentTaskWorkflowInspector["inspect"]>>;
    try {
      temporal = await this.workflows.inspect(task.taskId);
    } catch {
      return { taskId: task.taskId, outcome: "unavailable", reason: "temporal_query" };
    }
    if (temporal.state.taskId !== task.taskId || temporal.workflowId !== `dipole-agent-task/${task.taskId}`) {
      return { taskId: task.taskId, outcome: "conflict", reason: "workflow_binding" };
    }
    const projection = task.workflow;
    if (projection === undefined) {
      return { taskId: task.taskId, outcome: "missing", temporalStatus: temporal.state.status, temporalRevision: temporal.state.revision };
    }
    const evidence = {
      projectedStatus: projection.workflowStatus,
      projectedRevision: projection.workflowRevision,
      temporalStatus: temporal.state.status,
      temporalRevision: temporal.state.revision
    };
    if (projection.workflowId !== temporal.workflowId || projection.workflowRunId !== temporal.workflowRunId) {
      return { taskId: task.taskId, outcome: "conflict", reason: "workflow_binding", ...evidence };
    }
    if (projection.workflowStatus === temporal.state.status && projection.workflowRevision === temporal.state.revision) {
      return { taskId: task.taskId, outcome: "match" };
    }
    if (projection.workflowRevision < temporal.state.revision) return { taskId: task.taskId, outcome: "stale", ...evidence };
    if (projection.workflowRevision > temporal.state.revision) return { taskId: task.taskId, outcome: "ahead", ...evidence };
    return { taskId: task.taskId, outcome: "conflict", reason: "state", ...evidence };
  }
}

function boundedInteger(value: number, minimum: number, maximum: number, name: string): number {
  if (!Number.isSafeInteger(value) || value < minimum || value > maximum) {
    throw new Error(`Agent projection reconciliation ${name} must be between ${minimum} and ${maximum}`);
  }
  return value;
}
