import { describe, expect, it, vi } from "vitest";

import type { ExternalMcpDeploymentPlan } from "../mcp/external-mcp-deployment-composition.js";
import {
  loadShadowRuntimeConfig,
  type ShadowSubscriptionMatcher
} from "../runtime/shadow-runtime.js";
import type { AgentTaskWorkerActivities } from "./agent-task-activities.js";
import type { ExternalMcpTemporalClientLifecycle } from "./external-mcp-temporal-client-lifecycle.js";
import type { ExternalMcpTemporalWorkerComposition } from "./external-mcp-temporal-worker-composition.js";
import type { ExternalMcpTemporalWorkerLifecycle } from "./external-mcp-temporal-worker-lifecycle.js";
import {
  startExternalMcpShadowTemporalRuntime,
  type ExternalMcpShadowTemporalRuntimeSeams
} from "./external-mcp-shadow-temporal-runtime.js";
import type { TemporalMcpShadowRouteSelector } from "./mcp-shadow-task-dispatcher.js";
import { loadTemporalRuntimeConfig } from "./temporal-runtime.js";

describe("external MCP Shadow Temporal process owner", () => {
  it("keeps a disabled Worker bootstrap free of Client side effects", async () => {
    const harness = runtimeHarness();
    harness.startWorker.mockResolvedValueOnce(undefined);

    await expect(startExternalMcpShadowTemporalRuntime(
      {}, harness.shadow, harness.temporal, harness.activities, harness.createRoutes,
      {}, undefined, harness.seams
    )).resolves.toBeUndefined();

    expect(harness.startWorker).toHaveBeenCalledOnce();
    expect(harness.startClient).not.toHaveBeenCalled();
  });

  it("hands the exact Worker owner to Client and stops Client before Worker once", async () => {
    const harness = runtimeHarness();
    const controller = new AbortController();
    const onFailure = vi.fn();
    const env = { DIPOLE_AGENT_EXTERNAL_MCP_ENABLED: "true" };

    const runtime = await startExternalMcpShadowTemporalRuntime(
      env, harness.shadow, harness.temporal, harness.activities, harness.createRoutes,
      { signal: controller.signal }, onFailure, harness.seams
    );

    expect(harness.startWorker).toHaveBeenCalledWith(
      env,
      harness.shadow,
      harness.temporal,
      harness.activities,
      { signal: controller.signal },
      onFailure
    );
    expect(harness.startClient).toHaveBeenCalledWith(
      harness.worker,
      harness.createRoutes,
      { signal: controller.signal }
    );
    expect(runtime).toMatchObject({
      deployment: harness.worker.deployment,
      worker: harness.worker.worker,
      temporal: harness.worker.temporal,
      subscriptionMatcher: harness.subscriptionMatcher
    });

    await runtime!.dispatch({} as never, {} as never, "TASK-1");
    expect(harness.dispatch).toHaveBeenCalledOnce();
    await runtime!.stop();
    await runtime!.stop();
    expect(harness.order).toEqual(["client-stop", "worker-stop"]);
    expect(harness.clientStop).toHaveBeenCalledOnce();
    expect(harness.workerStop).toHaveBeenCalledOnce();
  });

  it("propagates pre-start cancellation without constructing Worker or Client", async () => {
    const harness = runtimeHarness();
    const controller = new AbortController();
    controller.abort(new Error("cancelled before process startup"));

    await expect(startExternalMcpShadowTemporalRuntime(
      {}, harness.shadow, harness.temporal, harness.activities, harness.createRoutes,
      { signal: controller.signal }, undefined, harness.seams
    )).rejects.toThrow("cancelled before process startup");
    expect(harness.startWorker).not.toHaveBeenCalled();
    expect(harness.startClient).not.toHaveBeenCalled();
  });

  it("returns the Worker when Client startup fails", async () => {
    const harness = runtimeHarness();
    harness.startClient.mockRejectedValueOnce(new Error("sensitive Client target"));

    await expect(startExternalMcpShadowTemporalRuntime(
      {}, harness.shadow, harness.temporal, harness.activities, harness.createRoutes,
      {}, undefined, harness.seams
    )).rejects.toThrow(/^External MCP Shadow Temporal runtime startup failed$/);
    expect(harness.order).toEqual(["worker-stop"]);
  });

  it("reports cleanup failure when Client startup and Worker rollback both fail", async () => {
    const harness = runtimeHarness({ workerStopError: new Error("sensitive Worker target") });
    harness.startClient.mockRejectedValueOnce(new Error("sensitive Client target"));

    await expect(startExternalMcpShadowTemporalRuntime(
      {}, harness.shadow, harness.temporal, harness.activities, harness.createRoutes,
      {}, undefined, harness.seams
    )).rejects.toThrow(/^External MCP Shadow Temporal runtime cleanup failed$/);
    expect(harness.workerStop).toHaveBeenCalledOnce();
  });

  it("closes Client then Worker when cancellation wins after ownership handoff", async () => {
    const harness = runtimeHarness();
    const controller = new AbortController();
    harness.startClient.mockImplementationOnce(async () => {
      controller.abort(new Error("cancelled after Client handoff"));
      return harness.client;
    });

    await expect(startExternalMcpShadowTemporalRuntime(
      {}, harness.shadow, harness.temporal, harness.activities, harness.createRoutes,
      { signal: controller.signal }, undefined, harness.seams
    )).rejects.toThrow("cancelled after Client handoff");
    expect(harness.order).toEqual(["client-stop", "worker-stop"]);
  });

  it("continues Worker cleanup after Client shutdown failure and memoizes rejection", async () => {
    const harness = runtimeHarness({ clientStopError: new Error("sensitive Client close") });
    const runtime = await startExternalMcpShadowTemporalRuntime(
      {}, harness.shadow, harness.temporal, harness.activities, harness.createRoutes,
      {}, undefined, harness.seams
    );

    await expect(runtime!.stop()).rejects.toThrow(/^External MCP Shadow Temporal runtime shutdown failed$/);
    await expect(runtime!.stop()).rejects.toThrow(/^External MCP Shadow Temporal runtime shutdown failed$/);
    expect(harness.order).toEqual(["client-stop", "worker-stop"]);
    expect(harness.clientStop).toHaveBeenCalledOnce();
    expect(harness.workerStop).toHaveBeenCalledOnce();
  });
});

function runtimeHarness(options: {
  readonly clientStopError?: Error;
  readonly workerStopError?: Error;
} = {}) {
  const order: string[] = [];
  const shadow = loadShadowRuntimeConfig({});
  const temporal = loadTemporalRuntimeConfig({ DIPOLE_AGENT_TEMPORAL_ENABLED: "true" });
  const activities = { executeAgentTaskStep: vi.fn() } as unknown as AgentTaskWorkerActivities;
  const deployment = {} as ExternalMcpDeploymentPlan;
  const composition = {} as ExternalMcpTemporalWorkerComposition;
  const subscriptionMatcher = {} as ShadowSubscriptionMatcher;
  const workerStop = vi.fn(async () => {
    order.push("worker-stop");
    if (options.workerStopError !== undefined) throw options.workerStopError;
  });
  const worker = {
    deployment,
    worker: composition,
    temporal: Object.freeze({ ...temporal }),
    subscriptionMatcher,
    stop: workerStop
  } satisfies ExternalMcpTemporalWorkerLifecycle;
  const dispatch = vi.fn(async () => undefined);
  const clientStop = vi.fn(async () => {
    order.push("client-stop");
    if (options.clientStopError !== undefined) throw options.clientStopError;
  });
  const client = { dispatch, stop: clientStop } satisfies ExternalMcpTemporalClientLifecycle;
  const createRoutes = vi.fn(() => ({} as TemporalMcpShadowRouteSelector));
  const startWorker = vi.fn<ExternalMcpShadowTemporalRuntimeSeams["startWorker"]>(async () => worker);
  const startClient = vi.fn<ExternalMcpShadowTemporalRuntimeSeams["startClient"]>(async () => client);
  const seams = { startWorker, startClient } satisfies ExternalMcpShadowTemporalRuntimeSeams;
  return {
    order, shadow, temporal, activities, worker, client, subscriptionMatcher, dispatch, workerStop, clientStop,
    createRoutes, startWorker, startClient, seams
  };
}
