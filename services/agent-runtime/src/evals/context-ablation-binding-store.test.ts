import { describe, expect, it } from "vitest";

import { assertExistingBinding, assertRunTarget } from "./context-ablation-binding-store.js";

const binding = { caseSha256: "a".repeat(64), condition: "baseline" as const, taskId: "task:one", runId: "run:one" };
const target = { task_uuid: "task:one", candidate_version: "agent@one", mode: "shadow", run_status: "completed", task_status: "completed" };

describe("Context ablation binding admission", () => {
  it("admits only completed shadow evidence for the exact task and candidate", () => {
    expect(() => assertRunTarget(target, binding, "agent@one")).not.toThrow();
    expect(() => assertRunTarget({ ...target, mode: "active" }, binding, "agent@one")).toThrow(/completed shadow/i);
    expect(() => assertRunTarget({ ...target, task_uuid: "task:other" }, binding, "agent@one")).toThrow(/planned task/i);
    expect(() => assertRunTarget({ ...target, candidate_version: "agent@two" }, binding, "agent@one")).toThrow(/candidate version/i);
  });

  it("only treats an exact prior admission as an idempotent replay", () => {
    expect(() => assertExistingBinding({ task_uuid: "task:one", run_uuid: "run:one", candidate_version: "agent@one" }, binding, "agent@one")).not.toThrow();
    expect(() => assertExistingBinding({ task_uuid: "task:other", run_uuid: "run:one", candidate_version: "agent@one" }, binding, "agent@one")).toThrow(/conflicts/i);
  });
});
