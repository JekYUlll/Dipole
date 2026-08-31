import type { Pool, RowDataPacket } from "mysql2/promise";

import type { ShadowEvalObservation, ShadowEvalObservationStore } from "./shadow-eval-adapter.js";

const listBindings = `SELECT case_sha256, condition_name, task_uuid, run_uuid, candidate_version
FROM agent_context_ablation_bindings WHERE experiment_uuid = ? ORDER BY case_sha256 ASC, condition_name ASC`;
const conditions = ["baseline", "retrieval", "memory"] as const;
type Condition = (typeof conditions)[number];

interface BindingRow extends RowDataPacket {
  case_sha256: string; condition_name: string; task_uuid: string; run_uuid: string; candidate_version: string;
}
export interface ContextAblationObservationStore { load(experimentId: string): Promise<readonly ContextAblationCaseObservation[]>; }
export interface ContextAblationCaseObservation { caseSha256: string; candidateVersion: string; observations: Readonly<Record<Condition, ShadowEvalObservation>>; }

export class MySQLContextAblationObservationStore implements ContextAblationObservationStore {
  constructor(private readonly pool: Pick<Pool, "execute">, private readonly observations: ShadowEvalObservationStore) {}
  async load(experimentId: string): Promise<readonly ContextAblationCaseObservation[]> {
    const [rows] = await this.pool.execute<BindingRow[]>(listBindings, [required(experimentId)]);
    const grouped = new Map<string, BindingRow[]>();
    for (const row of rows) grouped.set(row.case_sha256, [...(grouped.get(row.case_sha256) ?? []), row]);
    if (grouped.size === 0) throw new Error("Context ablation experiment is empty");
    return Promise.all([...grouped.entries()].map(async ([caseSha256, bindings]) => {
      if (bindings.length !== 3 || !conditions.every(name => bindings.some(row => row.condition_name === name))) throw new Error("Context ablation case is incomplete");
      if (new Set(bindings.map(row => row.candidate_version)).size !== 1) throw new Error("Context ablation candidate version drift");
      const entries = await Promise.all(bindings.map(async row => [row.condition_name, await this.observations.load(row.task_uuid, row.run_uuid)] as const));
      return { caseSha256, candidateVersion: bindings[0]!.candidate_version, observations: Object.fromEntries(entries) as Record<Condition, ShadowEvalObservation> };
    }));
  }
}
function required(value: string): string { value = value.trim(); if (!/^[A-Za-z0-9][A-Za-z0-9_.:-]{1,127}$/.test(value)) throw new Error("Context ablation experiment ID is invalid"); return value; }
