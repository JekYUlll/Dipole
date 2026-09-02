import { describe, expect, it } from "vitest";
import { MySQLContextAblationObservationStore } from "./mysql-context-ablation-store.js";

describe("MySQL context ablation observation store", () => {
  it("requires all three conditions before loading observations", async () => {
    const pool = { execute: async () => [[
      { case_sha256: "a".repeat(64), condition_name: "baseline", task_uuid: "TASK-1", run_uuid: "RUN-1", candidate_version: "agent@1" },
      { case_sha256: "a".repeat(64), condition_name: "retrieval", task_uuid: "TASK-2", run_uuid: "RUN-2", candidate_version: "agent@1" }
    ]] } as any;
    const store = new MySQLContextAblationObservationStore(pool, { load: async () => { throw new Error("must not load"); } });
    await expect(store.load("EXP-1")).rejects.toThrow(/incomplete/);
  });
});
