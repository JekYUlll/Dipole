import { describe, expect, it, vi } from "vitest";

import {
  createPersistentAgentTaskLifecycleActivities,
  type PersistentAgentRunLifecyclePort
} from "./agent-task-lifecycle-activities.js";

describe("Persistent Agent Task lifecycle Activities", () => {
  it("admits from trusted Workflow input and commits the exact terminal evidence", async () => {
    const admitRun = vi.fn(async () => ({ taskId: "task-1", runId: "run-1", runStatus: "running" as const }));
    const finish = vi.fn(async () => undefined);
    const activities = createPersistentAgentTaskLifecycleActivities({ admitRun, finish } satisfies PersistentAgentRunLifecyclePort);

    await expect(activities.admitAgentTask({
      taskId: "task-1",
      goal: "compile a digest",
      admission: {
        tenantId: "dipole",
        principalUserId: "U100",
        agentId: "UAI",
        triggerType: "message.direct.created",
        triggerRef: "M1",
        eventId: "E1",
        requestId: "REQ1",
        traceId: "TRACE1"
      }
    })).resolves.toEqual({ taskId: "task-1", runId: "run-1", runStatus: "running" });
    expect(admitRun).toHaveBeenCalledWith({
      tenantId: "dipole",
      principalUserId: "U100",
      agentId: "UAI",
      triggerType: "message.direct.created",
      triggerRef: "M1",
      eventId: "E1",
      requestId: "REQ1",
      traceId: "TRACE1"
    });

    await activities.finishAgentTask({
      taskId: "task-1",
      runId: "run-1",
      runStatus: "failed",
      lastError: "Activity retries exhausted",
      requestId: "REQ1",
      traceId: "TRACE1"
    });
    expect(finish).toHaveBeenCalledWith(
      "task-1", "run-1", "failed", "Activity retries exhausted",
      { requestId: "REQ1", traceId: "TRACE1" }
    );
  });

  it("rejects a persistent Workflow without trusted admission data", async () => {
    const port: PersistentAgentRunLifecyclePort = {
      admitRun: vi.fn(),
      finish: vi.fn()
    };
    const activities = createPersistentAgentTaskLifecycleActivities(port);

    await expect(activities.admitAgentTask({ taskId: "task-1", goal: "digest" }))
      .rejects.toThrow(/admission/i);
  });
});
