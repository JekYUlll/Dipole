import type { Pool, PoolConnection, RowDataPacket } from "mysql2/promise";

import type { ContextAblationBindingPlan } from "./context-ablation-binding-plan.js";

type Binding = ContextAblationBindingPlan["bindings"][number];

interface ExistingBinding extends RowDataPacket {
  readonly task_uuid: string;
  readonly run_uuid: string;
  readonly candidate_version: string;
}

interface RunTarget extends RowDataPacket {
  readonly task_uuid: string;
  readonly candidate_version: string;
  readonly mode: string;
  readonly run_status: string;
  readonly task_status: string;
}

const selectBinding = `SELECT task_uuid, run_uuid, candidate_version
FROM agent_context_ablation_bindings
WHERE experiment_uuid = ? AND case_sha256 = ? AND condition_name = ? FOR UPDATE`;
const selectRunBinding = `SELECT experiment_uuid, case_sha256, condition_name, task_uuid, candidate_version
FROM agent_context_ablation_bindings WHERE run_uuid = ? FOR UPDATE`;
const selectRun = `SELECT r.task_uuid, r.candidate_version, r.mode, r.status AS run_status, t.status AS task_status
FROM agent_runs AS r JOIN agent_tasks AS t ON t.task_uuid = r.task_uuid
WHERE r.run_uuid = ? FOR UPDATE`;
const insertBinding = `INSERT INTO agent_context_ablation_bindings
  (experiment_uuid, case_sha256, condition_name, task_uuid, run_uuid, candidate_version)
VALUES (?, ?, ?, ?, ?, ?)`;

export interface ContextAblationBindingReceipt {
  readonly bindingCount: number;
  readonly insertedCount: number;
  readonly replayedCount: number;
}

/** Writes a verified three-condition plan atomically with the operator-only account. */
export class MySQLContextAblationBindingStore {
  constructor(private readonly pool: Pick<Pool, "getConnection">) {}

  async apply(plan: ContextAblationBindingPlan): Promise<ContextAblationBindingReceipt> {
    const connection = await this.pool.getConnection();
    let committed = false;
    try {
      await connection.beginTransaction();
      let insertedCount = 0;
      let replayedCount = 0;
      for (const binding of plan.bindings) {
        const existing = await one<ExistingBinding>(connection, selectBinding, [plan.experimentId, binding.caseSha256, binding.condition]);
        if (existing !== undefined) {
          assertExistingBinding(existing, binding, plan.candidateVersion);
          replayedCount += 1;
          continue;
        }
        const boundRun = await one<RowDataPacket>(connection, selectRunBinding, [binding.runId]);
        if (boundRun !== undefined) throw new Error("Context ablation run is already bound to another experiment case");
        const target = await one<RunTarget>(connection, selectRun, [binding.runId]);
        assertRunTarget(target, binding, plan.candidateVersion);
        await connection.execute(insertBinding, [plan.experimentId, binding.caseSha256, binding.condition, binding.taskId, binding.runId, plan.candidateVersion]);
        insertedCount += 1;
      }
      await connection.commit();
      committed = true;
      return { bindingCount: plan.bindings.length, insertedCount, replayedCount };
    } finally {
      if (!committed) await connection.rollback().catch(() => undefined);
      connection.release();
    }
  }
}

export function assertExistingBinding(existing: Pick<ExistingBinding, "task_uuid" | "run_uuid" | "candidate_version">, binding: Binding, candidateVersion: string): void {
  if (existing.task_uuid !== binding.taskId || existing.run_uuid !== binding.runId || existing.candidate_version !== candidateVersion) {
    throw new Error("Context ablation binding conflicts with an existing admission");
  }
}

export function assertRunTarget(target: Pick<RunTarget, "task_uuid" | "candidate_version" | "mode" | "run_status" | "task_status"> | undefined, binding: Binding, candidateVersion: string): void {
  if (target === undefined) throw new Error("Context ablation run was not found");
  if (target.task_uuid !== binding.taskId) throw new Error("Context ablation run does not belong to the planned task");
  if (target.candidate_version !== candidateVersion) throw new Error("Context ablation run candidate version drift");
  if (target.mode !== "shadow" || target.run_status !== "completed" || target.task_status !== "completed") {
    throw new Error("Context ablation run and task must be completed shadow evidence");
  }
}

async function one<T extends RowDataPacket>(connection: Pick<PoolConnection, "execute">, query: string, values: readonly string[]): Promise<T | undefined> {
  // mysql2 loses the generic SQL-string overload when PoolConnection is narrowed.
  const execute = connection.execute as unknown as (sql: string, parameters: readonly string[]) => Promise<[T[], unknown]>;
  const [rows] = await execute(query, values);
  if (rows.length > 1) throw new Error("Context ablation admission expected one locked row");
  return rows[0];
}
