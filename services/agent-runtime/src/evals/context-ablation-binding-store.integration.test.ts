import { randomUUID } from "node:crypto";
import { readFile } from "node:fs/promises";

import { createPool, type Pool } from "mysql2/promise";
import { afterAll, beforeAll, describe, expect, it } from "vitest";

import { buildContextAblationBindingPlan } from "./context-ablation-binding-plan.js";
import { MySQLContextAblationBindingStore } from "./context-ablation-binding-store.js";

const adminUrl = process.env.DIPOLE_TEST_AGENT_MYSQL_URL;
const integration = describe.skipIf(adminUrl === undefined);
const hash = "a".repeat(64);

integration("Context ablation binding MySQL 8.4 contract", () => {
  const database = `dipole_context_binding_${randomUUID().replaceAll("-", "")}`;
  let admin: Pool;
  let pool: Pool;

  beforeAll(async () => {
    admin = createPool({ uri: adminUrl!, timezone: "Z" });
    await admin.query(`CREATE DATABASE \`${database}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci`);
    const uri = new URL(adminUrl!);
    uri.pathname = `/${database}`;
    pool = createPool({ uri: uri.toString(), timezone: "Z", multipleStatements: true });
    for (const number of [16, 17, 20, 21, 32, 56, 63]) {
      const prefix = number.toString().padStart(6, "0");
      await pool.query(await readFile(new URL(`../../../../db/migrations/${prefix}_${names[number]}.up.sql`, import.meta.url), "utf8"));
    }
    await seed(pool);
  });

  afterAll(async () => {
    await pool?.end();
    if (admin !== undefined) { await admin.query(`DROP DATABASE IF EXISTS \`${database}\``); await admin.end(); }
  });

  it("binds completed shadow runs once and atomically rejects a mixed invalid plan", async () => {
    const store = new MySQLContextAblationBindingStore(pool);
    const first = await store.apply(plan());
    expect(first).toEqual({ bindingCount: 3, insertedCount: 3, replayedCount: 0 });
    await expect(store.apply(plan())).resolves.toEqual({ bindingCount: 3, insertedCount: 0, replayedCount: 3 });
    const invalid = plan("experiment:two", ["RUN-4", "RUN-5", "RUN-6"]);
    await expect(store.apply(invalid)).rejects.toThrow(/completed shadow/i);
    const [rows] = await pool.query<Array<{ experiment_uuid: string } & import("mysql2").RowDataPacket>>("SELECT experiment_uuid FROM agent_context_ablation_bindings ORDER BY experiment_uuid");
    expect(rows).toHaveLength(3);
    expect(rows.every(row => row.experiment_uuid === "experiment:one")).toBe(true);
  });
});

const names: Record<number, string> = { 16: "agent_policy_persistence", 17: "agent_policy_identity_width", 20: "agent_shadow_trajectory", 21: "agent_runs", 32: "agent_runtime_promotion_grants", 56: "agent_context_ablation_bindings", 63: "agent_run_attempts" };

async function seed(pool: Pool): Promise<void> {
  for (let index = 1; index <= 6; index += 1) {
    const task = `TASK-${index}`;
    const run = `RUN-${index}`;
    const status = index === 6 ? "failed" : "completed";
    await pool.execute("INSERT INTO agent_tasks (task_uuid, definition_uuid, definition_version, tenant_id, principal_uuid, agent_uuid, status, trigger_type, trigger_ref, goal) VALUES (?, 'DEF-1', 1, 'dipole', 'U1', 'AI1', ?, 'test', ?, 'evaluate')", [task, status, task]);
    await pool.execute("INSERT INTO agent_runs (run_uuid, task_uuid, runtime_id, candidate_version, mode, status, started_at, completed_at) VALUES (?, ?, ?, 'agent@one', 'shadow', ?, UTC_TIMESTAMP(3), UTC_TIMESTAMP(3))", [run, task, `runtime-${index}`, status]);
  }
}

function plan(experimentId = "experiment:one", runs = ["RUN-1", "RUN-2", "RUN-3"]) {
  return buildContextAblationBindingPlan({ schemaVersion: "dipole.agent.context-ablation-manifest.v1", experimentId, candidateVersion: "agent@one", routePrices: [{ route: "test", inputMicrousdPerMillionTokens: 1, outputMicrousdPerMillionTokens: 1 }], cases: [{ caseSha256: hash, requiredOutputIds: ["artifact:test:v1"], relevantEvidenceIds: ["evidence:test"] }] }, { schemaVersion: "dipole.agent.context-ablation-binding-plan.v1", experimentId, candidateVersion: "agent@one", bindings: ["baseline", "retrieval", "memory"].map((condition, index) => ({ caseSha256: hash, condition, taskId: `TASK-${index + 1 + (experimentId === "experiment:two" ? 3 : 0)}`, runId: runs[index]! })) });
}
