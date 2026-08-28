import { describe, expect, it, vi } from "vitest";

import type { ExecutionContext } from "../runtime/execution-context.js";
import type { McpWorkerDispatchCheckpointV1 } from "../mcp/mcp-worker-dispatch.js";
import {
  TemporalMcpDispatchActivity,
  createTemporalMcpDispatchActivities,
  temporalMcpDispatchRouteBinding,
  type TemporalMcpContextResolver,
  type TemporalMcpInvocationProducer,
  type TemporalMcpResultProjector,
  type TemporalMcpTerminalWorker
} from "./mcp-dispatch-activity.js";

const context: ExecutionContext = {
  tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId: "TASK-1", runId: "RUN-1",
  mode: "shadow", permissions: ["calendar.read"],
  resourceScopes: [{ resourceType: "calendar", resourceId: "CAL-1", actions: ["read"] }],
  approvedCapabilities: [], requestId: "REQ-1", traceId: "TRACE-1"
};
const route = {
  routeId: "calendar-event-read", routeVersion: 1, capabilityId: "calendar.event.read",
  workflowStep: 3, ordinal: 1, deploymentBindingSha256: "d".repeat(64)
};
const routeBinding = temporalMcpDispatchRouteBinding(route);
const invocationId = "a".repeat(64);
const roundId = "b".repeat(64);

describe("Temporal MCP dispatch Activity", () => {
  it("registers as a dedicated Activity without changing the generic Worker mode", async () => {
    const dependencies = completedDependencies();
    const activities = createTemporalMcpDispatchActivities(route, dependencies);
    await expect(activities.executeMcpDispatch({
      kind: "begin", ...routeBinding, taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100", arguments: {}
    })).resolves.toMatchObject({ kind: "complete", output: { invocationId } });
  });

  it("binds route authority at construction and returns only an Artifact receipt", async () => {
    const dependencies = completedDependencies();
    const activity = new TemporalMcpDispatchActivity(route, dependencies);

    await expect(activity.execute({
      kind: "begin", ...routeBinding, taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100",
      arguments: { calendarId: "CAL-1", eventId: "EV-1" }, requestId: "REQ-1", traceId: "TRACE-1"
    })).resolves.toEqual({
      kind: "complete",
      output: { invocationId, roundId, artifactId: "c".repeat(64), artifactVersion: 1 }
    });

    expect(dependencies.contexts.resolveMcpContext).toHaveBeenCalledWith(
      "TASK-1", "RUN-1", "U100", { requestId: "REQ-1", traceId: "TRACE-1" }
    );
    expect(dependencies.producer.produce).toHaveBeenCalledWith({
      workflowStep: 3, ordinal: 1, capabilityId: "calendar.event.read",
      arguments: { calendarId: "CAL-1", eventId: "EV-1" }
    }, context);
    expect(dependencies.worker.begin).toHaveBeenCalledWith({ taskId: "TASK-1", runId: "RUN-1", invocationId }, expect.any(AbortSignal));
    expect(dependencies.projector.project).toHaveBeenCalledWith({
      context, invocationId, roundId, result: { content: [], secretBody: "untrusted" }
    }, expect.any(AbortSignal));
  });

  it("re-resolves Context and exact-replays the producer before durable resume", async () => {
    const dependencies = completedDependencies();
    const durableWorkerCheckpoint = workerCheckpoint();
    dependencies.worker.begin.mockResolvedValueOnce({
      kind: "wait_input",
      checkpoint: durableWorkerCheckpoint,
      directive: {
        kind: "wait_input", requestId: "INPUT-1", prompt: "Choose",
        form: { schemaVersion: "dipole.agent.elicitation.v1", fields: [{ id: "choice", label: "Choice", type: "text", required: true }] },
        source: { kind: "mcp", serverId: "calendar.example", toolName: "calendar.read", invocationId, trust: "untrusted" },
        expiresAtUnixMs: 2_000,
        checkpoint: durableWorkerCheckpoint
      }
    });
    const activity = new TemporalMcpDispatchActivity(route, dependencies);
    const wait = await activity.execute({
      kind: "begin", ...routeBinding, taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100",
      arguments: { eventId: "EV-1", calendarId: "CAL-1" }, requestId: "REQ-1", traceId: "TRACE-1"
    });
    if (wait.kind !== "wait_input") throw new Error("expected wait_input");

    await expect(activity.execute({
      kind: "resume",
      checkpoint: wait.checkpoint,
      resume: { kind: "input", requestId: "INPUT-1", value: { choice: "yes" } }
    })).resolves.toMatchObject({ kind: "complete", output: { invocationId, artifactId: "c".repeat(64) } });

    expect(dependencies.contexts.resolveMcpContext).toHaveBeenCalledTimes(2);
    expect(dependencies.producer.produce).toHaveBeenCalledTimes(2);
    expect(dependencies.producer.produce.mock.calls[1]?.[0]).toEqual({
      workflowStep: 3, ordinal: 1, capabilityId: "calendar.event.read",
      arguments: { calendarId: "CAL-1", eventId: "EV-1" }
    });
    expect(dependencies.worker.resume).toHaveBeenCalledWith(
      durableWorkerCheckpoint,
      { action: "accept", resume: { kind: "input", requestId: "INPUT-1", value: { choice: "yes" } } },
      expect.any(AbortSignal)
    );
  });

  it("rejects caller authority, checkpoint drift, and cancellation before Core access", async () => {
    const dependencies = completedDependencies();
    const activity = new TemporalMcpDispatchActivity(route, dependencies);
    await expect(activity.execute({
      kind: "begin", ...routeBinding, taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100", arguments: {},
      capabilityId: "forged", profileId: "forged"
    })).rejects.toThrow(/input is invalid/i);
    expect(dependencies.contexts.resolveMcpContext).not.toHaveBeenCalled();
    await expect(activity.execute({
      kind: "begin", ...routeBinding, routeVersion: 2,
      taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100", arguments: {}
    })).rejects.toThrow(/route binding/i);
    expect(dependencies.contexts.resolveMcpContext).not.toHaveBeenCalled();
    await expect(new TemporalMcpDispatchActivity({ ...route, ordinal: 2 }, dependencies).execute({
      kind: "begin", ...routeBinding,
      taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100", arguments: {}
    })).rejects.toThrow(/route binding/i);
    await expect(new TemporalMcpDispatchActivity({ ...route, capabilityId: "calendar.event.list" }, dependencies).execute({
      kind: "begin", ...routeBinding,
      taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100", arguments: {}
    })).rejects.toThrow(/route binding/i);
    await expect(new TemporalMcpDispatchActivity({ ...route, deploymentBindingSha256: "e".repeat(64) }, dependencies).execute({
      kind: "begin", ...routeBinding,
      taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100", arguments: {}
    })).rejects.toThrow(/route binding/i);
    expect(dependencies.contexts.resolveMcpContext).not.toHaveBeenCalled();

    const waiting = completedDependencies();
    const durableWorkerCheckpoint = workerCheckpoint();
    waiting.worker.begin.mockResolvedValueOnce({
      kind: "wait_input", checkpoint: durableWorkerCheckpoint,
      directive: {
        kind: "wait_input", requestId: "INPUT-1", prompt: "Choose",
        form: { schemaVersion: "dipole.agent.elicitation.v1", fields: [{ id: "choice", label: "Choice", type: "text", required: true }] },
        expiresAtUnixMs: 2_000, checkpoint: durableWorkerCheckpoint
      }
    });
    const wait = await new TemporalMcpDispatchActivity(route, waiting).execute({
      kind: "begin", ...routeBinding, taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100", arguments: {}
    });
    if (wait.kind !== "wait_input") throw new Error("expected wait_input");
    await expect(new TemporalMcpDispatchActivity({ ...route, ordinal: 2 }, waiting).execute({
      kind: "resume", checkpoint: wait.checkpoint,
      resume: { kind: "input", requestId: "INPUT-1", value: { choice: "yes" } }
    })).rejects.toThrow(/checkpoint binding/i);

    const controller = new AbortController();
    controller.abort(new Error("cancelled before dispatch"));
    const cancelled = completedDependencies(() => controller.signal);
    await expect(new TemporalMcpDispatchActivity(route, cancelled).execute({
      kind: "begin", ...routeBinding, taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100", arguments: {}
    })).rejects.toThrow(/cancelled before dispatch/i);
    expect(cancelled.contexts.resolveMcpContext).not.toHaveBeenCalled();
  });

  it("replays the same terminal lineage after Activity completion loss", async () => {
    const dependencies = completedDependencies();
    const activity = new TemporalMcpDispatchActivity(route, dependencies);
    const input = {
      kind: "begin" as const, ...routeBinding, taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100",
      arguments: { calendarId: "CAL-1", eventId: "EV-1" }
    };

    const first = await activity.execute(input);
    const replay = await activity.execute(input);

    expect(replay).toEqual(first);
    expect(dependencies.contexts.resolveMcpContext).toHaveBeenCalledTimes(2);
    expect(dependencies.producer.produce).toHaveBeenCalledTimes(2);
    expect(dependencies.worker.begin).toHaveBeenCalledTimes(2);
    expect(dependencies.projector.project).toHaveBeenCalledTimes(2);
    expect(dependencies.projector.project.mock.calls[1]?.[0]).toEqual(dependencies.projector.project.mock.calls[0]?.[0]);
  });

  it("projects a terminal begin receipt without exposing the external result", async () => {
    const dependencies = completedDependencies();
    dependencies.producer.produce.mockResolvedValueOnce({
      invocationId, status: "completed", taskId: "TASK-1", runId: "RUN-1"
    });
    const activity = new TemporalMcpDispatchActivity(route, dependencies);

    const result = await activity.execute({
      kind: "begin", ...routeBinding, taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100",
      arguments: { calendarId: "CAL-1", eventId: "EV-1" }
    });

    expect(result).toEqual({
      kind: "complete",
      output: { invocationId, roundId, artifactId: "c".repeat(64), artifactVersion: 1 }
    });
    expect(result).not.toHaveProperty("output.result");
    expect(dependencies.worker.begin).toHaveBeenCalledWith(
      { taskId: "TASK-1", runId: "RUN-1", invocationId }, expect.any(AbortSignal)
    );
  });

  it("stops after a cancellation observed during fresh Context resolution", async () => {
    const controller = new AbortController();
    const dependencies = completedDependencies(() => controller.signal);
    dependencies.contexts.resolveMcpContext.mockImplementationOnce(async () => {
      controller.abort(new Error("cancelled during Context resolution"));
      return context;
    });
    const activity = new TemporalMcpDispatchActivity(route, dependencies);

    await expect(activity.execute({
      kind: "begin", ...routeBinding, taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100", arguments: {}
    })).rejects.toThrow(/cancelled during Context resolution/i);
    expect(dependencies.producer.produce).not.toHaveBeenCalled();
    expect(dependencies.worker.begin).not.toHaveBeenCalled();
  });

  it("does not commit a directive after cancellation during terminal dispatch", async () => {
    const controller = new AbortController();
    const dependencies = completedDependencies(() => controller.signal);
    dependencies.worker.begin.mockImplementationOnce(async () => {
      controller.abort(new Error("cancelled during terminal dispatch"));
      return {
        kind: "complete", result: { content: [] }, receipt: { roundId, roundNumber: 0 }
      };
    });
    const activity = new TemporalMcpDispatchActivity(route, dependencies);

    await expect(activity.execute({
      kind: "begin", ...routeBinding, taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100", arguments: {}
    })).rejects.toThrow(/cancelled during terminal dispatch/i);
    expect(dependencies.projector.project).not.toHaveBeenCalled();
  });
});

function completedDependencies(cancellationSignal: () => AbortSignal = () => new AbortController().signal) {
  const resolveMcpContext = vi.fn<TemporalMcpContextResolver["resolveMcpContext"]>(async () => context);
  const produce = vi.fn<TemporalMcpInvocationProducer["produce"]>(async () => ({
    invocationId, status: "running", taskId: "TASK-1", runId: "RUN-1"
  }));
  const begin = vi.fn<TemporalMcpTerminalWorker["begin"]>(async () => ({
    kind: "complete", result: { content: [], secretBody: "untrusted" }, receipt: { roundId, roundNumber: 0 }
  }));
  const resume = vi.fn<TemporalMcpTerminalWorker["resume"]>(async () => ({
    kind: "complete", result: { content: [], secretBody: "resumed" }, receipt: { roundId, roundNumber: 1 }
  }));
  const project = vi.fn<TemporalMcpResultProjector["project"]>(async () => ({
    artifactId: "c".repeat(64), artifactVersion: 1
  }));
  return {
    contexts: { resolveMcpContext },
    producer: { produce },
    worker: { begin, resume },
    projector: { project },
    cancellationSignal
  };
}

function workerCheckpoint(): McpWorkerDispatchCheckpointV1 {
  return {
    taskId: "TASK-1", runId: "RUN-1", invocationId, worker: true
  } as unknown as McpWorkerDispatchCheckpointV1;
}
