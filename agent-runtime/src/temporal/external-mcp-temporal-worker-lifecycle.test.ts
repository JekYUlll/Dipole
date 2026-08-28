import { describe, expect, it, vi } from "vitest";

import type { ExternalMcpDeploymentPlan } from "../mcp/external-mcp-deployment-composition.js";
import type { ExternalMcpTemporalWorkerComposition } from "./external-mcp-temporal-worker-composition.js";
import type { ExternalMcpTemporalWorkerStartupPlan } from "./external-mcp-temporal-worker-startup-plan.js";
import {
  startExternalMcpTemporalWorkerLifecycle,
  type ExternalMcpTemporalWorkerRuntimeFactory
} from "./external-mcp-temporal-worker-lifecycle.js";
import { loadTemporalRuntimeConfig, type TemporalWorkerRuntime } from "./temporal-runtime.js";

describe("external MCP Temporal Worker lifecycle", () => {
  it("keeps a disabled startup snapshot free of Worker side effects", async () => {
    const createRuntime = vi.fn<ExternalMcpTemporalWorkerRuntimeFactory>();

    await expect(startExternalMcpTemporalWorkerLifecycle(
      undefined,
      loadTemporalRuntimeConfig({}),
      undefined,
      createRuntime
    )).resolves.toBeUndefined();

    expect(createRuntime).not.toHaveBeenCalled();
  });

  it("starts the exact Worker activities and stops Worker before owned resources once", async () => {
    const order: string[] = [];
    const { startup, close } = startupPlan(order);
    const runtime = workerRuntime(order);
    const createRuntime = vi.fn<ExternalMcpTemporalWorkerRuntimeFactory>(() => runtime.value);
    const onFailure = vi.fn();
    const config = loadTemporalRuntimeConfig({ DIPOLE_AGENT_TEMPORAL_ENABLED: "true" });

    const lifecycle = await startExternalMcpTemporalWorkerLifecycle(
      startup,
      config,
      onFailure,
      createRuntime
    );

    expect(lifecycle).toMatchObject({ deployment: startup.deployment, worker: startup.worker });
    expect(createRuntime).toHaveBeenCalledWith(config, startup.worker.activities, onFailure);
    expect(runtime.start).toHaveBeenCalledOnce();

    await lifecycle!.stop();
    await lifecycle!.stop();

    expect(order).toEqual(["start", "worker-stop", "resource-close"]);
    expect(runtime.stop).toHaveBeenCalledOnce();
    expect(close).toHaveBeenCalledOnce();
  });

  it("rejects a disabled Temporal config before Worker construction and releases resources", async () => {
    const order: string[] = [];
    const { startup, close } = startupPlan(order);
    const createRuntime = vi.fn<ExternalMcpTemporalWorkerRuntimeFactory>();

    await expect(startExternalMcpTemporalWorkerLifecycle(
      startup,
      loadTemporalRuntimeConfig({}),
      undefined,
      createRuntime
    )).rejects.toThrow(/^External MCP Temporal Worker lifecycle startup failed$/);

    expect(createRuntime).not.toHaveBeenCalled();
    expect(order).toEqual(["resource-close"]);
    expect(close).toHaveBeenCalledOnce();
  });

  it("releases owned resources when the Worker Runtime factory throws", async () => {
    const order: string[] = [];
    const { startup, close } = startupPlan(order);

    await expect(startExternalMcpTemporalWorkerLifecycle(
      startup,
      loadTemporalRuntimeConfig({ DIPOLE_AGENT_TEMPORAL_ENABLED: "true" }),
      undefined,
      () => { throw new Error("sensitive Worker factory target"); }
    )).rejects.toThrow(/^External MCP Temporal Worker lifecycle startup failed$/);

    expect(order).toEqual(["resource-close"]);
    expect(close).toHaveBeenCalledOnce();
  });

  it("rolls back Worker and owned resources when Worker startup fails", async () => {
    const order: string[] = [];
    const { startup, close } = startupPlan(order);
    const runtime = workerRuntime(order, { startError: new Error("sensitive Temporal target") });

    await expect(startExternalMcpTemporalWorkerLifecycle(
      startup,
      loadTemporalRuntimeConfig({ DIPOLE_AGENT_TEMPORAL_ENABLED: "true" }),
      undefined,
      () => runtime.value
    )).rejects.toThrow(/^External MCP Temporal Worker lifecycle startup failed$/);

    expect(order).toEqual(["start", "worker-stop", "resource-close"]);
    expect(runtime.stop).toHaveBeenCalledOnce();
    expect(close).toHaveBeenCalledOnce();
  });

  it("reports a fixed cleanup failure when startup rollback cannot release resources", async () => {
    const order: string[] = [];
    const { startup } = startupPlan(order, new Error("sensitive RPC close failure"));
    const runtime = workerRuntime(order, { startError: new Error("sensitive startup failure") });

    await expect(startExternalMcpTemporalWorkerLifecycle(
      startup,
      loadTemporalRuntimeConfig({ DIPOLE_AGENT_TEMPORAL_ENABLED: "true" }),
      undefined,
      () => runtime.value
    )).rejects.toThrow(/^External MCP Temporal Worker lifecycle cleanup failed$/);

    expect(order).toEqual(["start", "worker-stop", "resource-close"]);
  });

  it("continues resource cleanup after Worker shutdown failure and memoizes rejection", async () => {
    const order: string[] = [];
    const { startup, close } = startupPlan(order);
    const runtime = workerRuntime(order, { stopError: new Error("sensitive Worker failure") });
    const lifecycle = await startExternalMcpTemporalWorkerLifecycle(
      startup,
      loadTemporalRuntimeConfig({ DIPOLE_AGENT_TEMPORAL_ENABLED: "true" }),
      undefined,
      () => runtime.value
    );

    await expect(lifecycle!.stop()).rejects.toThrow(/^External MCP Temporal Worker lifecycle shutdown failed$/);
    await expect(lifecycle!.stop()).rejects.toThrow(/^External MCP Temporal Worker lifecycle shutdown failed$/);

    expect(order).toEqual(["start", "worker-stop", "resource-close"]);
    expect(runtime.stop).toHaveBeenCalledOnce();
    expect(close).toHaveBeenCalledOnce();
  });

  it("reports resource close failure without repeating either shutdown stage", async () => {
    const order: string[] = [];
    const { startup, close } = startupPlan(order, new Error("sensitive resource failure"));
    const runtime = workerRuntime(order);
    const lifecycle = await startExternalMcpTemporalWorkerLifecycle(
      startup,
      loadTemporalRuntimeConfig({ DIPOLE_AGENT_TEMPORAL_ENABLED: "true" }),
      undefined,
      () => runtime.value
    );

    await expect(lifecycle!.stop()).rejects.toThrow(/^External MCP Temporal Worker lifecycle shutdown failed$/);
    await expect(lifecycle!.stop()).rejects.toThrow(/^External MCP Temporal Worker lifecycle shutdown failed$/);

    expect(order).toEqual(["start", "worker-stop", "resource-close"]);
    expect(runtime.stop).toHaveBeenCalledOnce();
    expect(close).toHaveBeenCalledOnce();
  });
});

function startupPlan(order: string[], closeError?: Error) {
  const close = vi.fn(async () => {
    order.push("resource-close");
    if (closeError !== undefined) throw closeError;
  });
  const worker = {
    activities: Object.freeze({ executeAgentTaskStep: vi.fn() })
  } as unknown as ExternalMcpTemporalWorkerComposition;
  const startup = {
    deployment: { deployment: true } as unknown as ExternalMcpDeploymentPlan,
    worker,
    close
  } satisfies ExternalMcpTemporalWorkerStartupPlan;
  return { startup, close };
}

function workerRuntime(
  order: string[],
  errors: { readonly startError?: Error; readonly stopError?: Error } = {}
) {
  const start = vi.fn(async () => {
    order.push("start");
    if (errors.startError !== undefined) throw errors.startError;
  });
  const stop = vi.fn(async () => {
    order.push("worker-stop");
    if (errors.stopError !== undefined) throw errors.stopError;
  });
  return { value: { start, stop } satisfies TemporalWorkerRuntime, start, stop };
}
