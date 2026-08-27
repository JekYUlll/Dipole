import { describe, expect, it, vi } from "vitest";
import type { Span } from "@opentelemetry/api";
import type { AgentTelemetry } from "../observability/agent-telemetry.js";

import {
  createPersistentAgentTaskLifecycleActivities,
  type PersistentAgentRunLifecyclePort
} from "./agent-task-lifecycle-activities.js";

describe("Persistent Agent Task lifecycle Activities", () => {
  it("admits from trusted Workflow input and commits the exact terminal evidence", async () => {
    const admitRun = vi.fn(async () => ({ taskId: "task-1", runId: "run-1", runStatus: "running" as const }));
    const finish = vi.fn(async () => undefined);
    const requestApproval = vi.fn(async () => undefined);
    const resolveApproval = vi.fn(async () => undefined);
    const projectTaskWorkflowState = vi.fn(async () => ({}));
    const activities = createPersistentAgentTaskLifecycleActivities({ admitRun, finish, requestApproval, resolveApproval, projectTaskWorkflowState } satisfies PersistentAgentRunLifecyclePort);

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
    await activities.projectAgentTaskState({
      taskId: "task-1", runId: "run-1", workflowId: "dipole-agent-task/task-1", workflowRunId: "temporal-1",
      workflowStatus: "running", workflowRevision: 1, requestId: "REQ1", traceId: "TRACE1"
    });
    expect(projectTaskWorkflowState).toHaveBeenCalledWith({
      taskId: "task-1", runId: "run-1", workflowId: "dipole-agent-task/task-1", workflowRunId: "temporal-1",
      workflowStatus: "running", workflowRevision: 1
    }, { requestId: "REQ1", traceId: "TRACE1" });
  });

  it("rejects a persistent Workflow without trusted admission data", async () => {
    const port: PersistentAgentRunLifecyclePort = {
      admitRun: vi.fn(),
      finish: vi.fn(),
      requestApproval: vi.fn(),
      resolveApproval: vi.fn(),
      projectTaskWorkflowState: vi.fn()
    };
    const activities = createPersistentAgentTaskLifecycleActivities(port);

    await expect(activities.admitAgentTask({ taskId: "task-1", goal: "digest" }))
      .rejects.toThrow(/admission/i);
  });

  it("records Task and Approval activity boundaries", async () => {
    const names: string[] = [];
    const port: PersistentAgentRunLifecyclePort = {
      admitRun: vi.fn(async () => ({ taskId: "task-1", runId: "run-1", runStatus: "running" as const })),
      finish: vi.fn(async () => undefined), requestApproval: vi.fn(async () => undefined),
      resolveApproval: vi.fn(async () => undefined), projectTaskWorkflowState: vi.fn(async () => undefined)
    };
    const telemetry = {
      withSpan: vi.fn(async (name: string, _context: unknown, operation: (span: Span) => Promise<unknown>) => {
        names.push(name);
        return operation({ setAttribute: vi.fn() } as unknown as Span);
      })
    } as unknown as Pick<AgentTelemetry, "withSpan">;
    const activities = createPersistentAgentTaskLifecycleActivities(port, telemetry);
    await activities.admitAgentTask({
      taskId: "task-1", goal: "digest",
      admission: { tenantId: "dipole", principalUserId: "U1", agentId: "AI1", triggerType: "event", triggerRef: "E1", eventId: "E1" }
    });
    await activities.requestAgentTaskApproval({
      taskId: "task-1", runId: "run-1",
      approval: {
        approvalId: "APP-1", capabilityId: "message.send",
        resourceScope: { resourceType: "conversation", resourceId: "group:g1", actions: ["write"] },
        scopeSha256: "a".repeat(64), argumentsSha256: "b".repeat(64), nonceSha256: "c".repeat(64), expiresAtUnixMs: 1
      }
    });
    await activities.resolveAgentTaskApproval({
      taskId: "task-1", runId: "run-1", approvalId: "APP-1", decision: "denied", actorUserId: "U1"
    });
    await activities.finishAgentTask({ taskId: "task-1", runId: "run-1", runStatus: "cancelled", lastError: "denied" });

    expect(names).toEqual(["agent.task", "agent.approval", "agent.approval", "agent.task"]);
  });
});
