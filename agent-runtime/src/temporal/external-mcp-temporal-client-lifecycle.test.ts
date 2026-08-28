import { describe, expect, it, vi } from "vitest";

import { agentTaskId, type AgentEvent, type AgentIdentity } from "../events/shadow-processor.js";
import {
  startExternalMcpTemporalClientLifecycle,
  type ExternalMcpTemporalClientResourceFactory
} from "./external-mcp-temporal-client-lifecycle.js";
import type { ExternalMcpTemporalWorkerComposition } from "./external-mcp-temporal-worker-composition.js";
import type { ExternalMcpTemporalWorkerLifecycle } from "./external-mcp-temporal-worker-lifecycle.js";
import { TemporalMcpSubscriptionRouteSelector } from "./mcp-subscription-route-selector.js";
import { TemporalMcpWorkflowExecutionCatalog } from "./mcp-workflow-envelope.js";
import type { TemporalRuntimeConfig } from "./temporal-runtime.js";

const binding = {
  routeId: "github-issue-read",
  routeVersion: 2,
  routeManifestSha256: "a".repeat(64)
};
const config: TemporalRuntimeConfig = {
  enabled: true,
  address: "127.0.0.1:7233",
  namespace: "default",
  taskQueue: "dipole-agent-task-v1",
  activityMode: "persistent_shadow"
};
const event: AgentEvent = {
  eventId: "EVENT-1",
  eventType: "message.direct.created",
  aggregateId: "MESSAGE-1",
  occurredAt: "2026-08-28T08:00:00.000Z",
  payload: { content: "inspect issue 42" },
  subscriptionId: "SUB-1",
  subscriptionBinding: {
    subscriptionId: "SUB-1",
    definitionId: "DEF-GUARDIAN",
    definitionVersion: 3,
    tenantId: "dipole",
    agentId: "UAI"
  }
};
const identity: AgentIdentity = {
  tenantId: "dipole",
  principalUuid: "U100",
  agentUuid: "UAI",
  requestId: "REQ-1"
};
const taskId = agentTaskId({
  tenantId: identity.tenantId,
  agentUuid: identity.agentUuid,
  triggerType: event.eventType,
  triggerRef: event.aggregateId
});

describe("external MCP Temporal Client lifecycle", () => {
  it("returns disabled without constructing routes or a Client resource", async () => {
    const createRoutes = vi.fn();
    const createResource = vi.fn();

    await expect(startExternalMcpTemporalClientLifecycle(undefined, createRoutes, { createResource }))
      .resolves.toBeUndefined();
    expect(createRoutes).not.toHaveBeenCalled();
    expect(createResource).not.toHaveBeenCalled();
  });

  it("starts trusted dispatch with the exact Worker Workflow catalog", async () => {
    const start = vi.fn(async () => ({ workflowId: `dipole-agent-task/${taskId}`, firstExecutionRunId: "RUN-1" }));
    const close = vi.fn(async () => undefined);
    const createResource = vi.fn(async () => ({ workflow: { start }, close }));
    const owner = workerLifecycle();
    const worker = owner.worker;
    const routes = routeSelector();
    const createRoutes = vi.fn(() => routes);

    const lifecycle = await startExternalMcpTemporalClientLifecycle(owner, createRoutes, { createResource });
    await lifecycle!.dispatch(event, identity, taskId);

    expect(createRoutes).toHaveBeenCalledWith(worker);
    expect(createResource).toHaveBeenCalledWith(config, expect.any(AbortSignal));
    expect(start).toHaveBeenCalledWith("agentTaskWorkflow", expect.objectContaining({
      taskQueue: config.taskQueue,
      args: [expect.objectContaining({
        taskId,
        execution: {
          kind: "external_mcp_v1",
          ...binding,
          arguments: { issue_number: 42, owner: "dipole", repo: "server" }
        }
      })]
    }));
    await lifecycle!.stop();
    await lifecycle!.stop();
    expect(close).toHaveBeenCalledOnce();
    await expect(lifecycle!.dispatch(event, identity, taskId)).rejects.toThrow(/not running/i);
  });

  it("rejects disabled Temporal before constructing routes or a Client", async () => {
    const createRoutes = vi.fn();
    const createResource = vi.fn();

    await expect(startExternalMcpTemporalClientLifecycle(
      workerLifecycle({ ...config, enabled: false }), createRoutes, { createResource }
    )).rejects.toThrow(/startup failed/i);
    expect(createRoutes).not.toHaveBeenCalled();
    expect(createResource).not.toHaveBeenCalled();
  });

  it("propagates pre-start cancellation without side effects", async () => {
    const controller = new AbortController();
    controller.abort(new Error("cancelled"));
    const createRoutes = vi.fn();
    const createResource = vi.fn();

    await expect(startExternalMcpTemporalClientLifecycle(
      workerLifecycle(), createRoutes, { signal: controller.signal, createResource }
    )).rejects.toThrow("cancelled");
    expect(createRoutes).not.toHaveBeenCalled();
    expect(createResource).not.toHaveBeenCalled();
  });

  it("closes a newly created Client when cancellation arrives during construction", async () => {
    const controller = new AbortController();
    const close = vi.fn(async () => undefined);
    const createResource: ExternalMcpTemporalClientResourceFactory = vi.fn(async () => {
      controller.abort(new Error("late cancellation"));
      return { workflow: { start: vi.fn() }, close };
    });

    await expect(startExternalMcpTemporalClientLifecycle(
      workerLifecycle(), () => routeSelector(), { signal: controller.signal, createResource }
    )).rejects.toThrow("late cancellation");
    expect(close).toHaveBeenCalledOnce();
  });

  it("keeps route and resource construction failures low-sensitive", async () => {
    const createResource = vi.fn();
    await expect(startExternalMcpTemporalClientLifecycle(
      workerLifecycle(), () => { throw new Error("route details"); }, { createResource }
    )).rejects.toThrow("External MCP Temporal Client lifecycle startup failed");
    expect(createResource).not.toHaveBeenCalled();

    await expect(startExternalMcpTemporalClientLifecycle(
      workerLifecycle(), () => routeSelector(), {
        createResource: vi.fn(async () => { throw new Error("target details"); })
      }
    )).rejects.toThrow("External MCP Temporal Client lifecycle startup failed");
  });

  it("memoizes a low-sensitive shutdown failure", async () => {
    const close = vi.fn(async () => { throw new Error("connection details"); });
    const lifecycle = await startExternalMcpTemporalClientLifecycle(
      workerLifecycle(), () => routeSelector(), {
        createResource: async () => ({ workflow: { start: vi.fn() }, close })
      }
    );

    await expect(lifecycle!.stop()).rejects.toThrow("External MCP Temporal Client lifecycle shutdown failed");
    await expect(lifecycle!.stop()).rejects.toThrow("External MCP Temporal Client lifecycle shutdown failed");
    expect(close).toHaveBeenCalledOnce();
  });

  it("drains an accepted Workflow start before closing the Client resource", async () => {
    let resolveStart: ((value: { workflowId: string }) => void) | undefined;
    const start = vi.fn(() => new Promise<{ workflowId: string }>((resolve) => { resolveStart = resolve; }));
    const close = vi.fn(async () => undefined);
    const lifecycle = await startExternalMcpTemporalClientLifecycle(
      workerLifecycle(), () => routeSelector(), {
        createResource: async () => ({ workflow: { start }, close })
      }
    );

    const dispatch = lifecycle!.dispatch(event, identity, taskId);
    const stop = lifecycle!.stop();
    await vi.waitFor(() => expect(start).toHaveBeenCalledOnce());
    expect(close).not.toHaveBeenCalled();

    resolveStart!({ workflowId: `dipole-agent-task/${taskId}` });
    await dispatch;
    await stop;
    expect(close).toHaveBeenCalledOnce();
  });
});

function workerComposition(): ExternalMcpTemporalWorkerComposition {
  return {
    activities: {} as ExternalMcpTemporalWorkerComposition["activities"],
    routeBindings: [binding],
    workflowExecutions: new TemporalMcpWorkflowExecutionCatalog([binding]),
    subscriptionRoutes: [],
    runtimeBindingSha256: "b".repeat(64)
  };
}

function workerLifecycle(temporal: TemporalRuntimeConfig = config): ExternalMcpTemporalWorkerLifecycle {
  return {
    deployment: {} as ExternalMcpTemporalWorkerLifecycle["deployment"],
    worker: workerComposition(),
    temporal: Object.freeze({ ...temporal }),
    stop: vi.fn(async () => undefined)
  };
}

function routeSelector(): TemporalMcpSubscriptionRouteSelector {
  return new TemporalMcpSubscriptionRouteSelector([{
    definitionId: "DEF-GUARDIAN",
    definitionVersion: 3,
    routeId: binding.routeId,
    resolveArguments: () => ({ owner: "dipole", repo: "server", issue_number: 42 })
  }]);
}
