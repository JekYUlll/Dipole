import { describe, expect, it, vi } from "vitest";

import type { ExternalMcpCapabilityDefinitionRegistry } from "../mcp/external-mcp-deployment-route-manifest.js";
import type { ExternalMcpDeploymentPlan } from "../mcp/external-mcp-deployment-composition.js";
import { loadShadowRuntimeConfig } from "../runtime/shadow-runtime.js";
import type { AgentTaskWorkerActivities } from "./agent-task-activities.js";
import type { ExternalMcpTemporalWorkerComposition } from "./external-mcp-temporal-worker-composition.js";
import type { ExternalMcpTemporalWorkerLifecycle } from "./external-mcp-temporal-worker-lifecycle.js";
import type {
  ExternalMcpTemporalWorkerResource,
  ExternalMcpTemporalWorkerStartupPlan
} from "./external-mcp-temporal-worker-startup-plan.js";
import {
  startExternalMcpShadowWorkerBootstrap,
  type ExternalMcpShadowWorkerBootstrapSeams
} from "./external-mcp-shadow-worker-bootstrap.js";
import { loadTemporalRuntimeConfig } from "./temporal-runtime.js";

describe("external MCP Shadow Worker bootstrap", () => {
  it("keeps disabled deployment free of RPC factory and Worker side effects", async () => {
    const harness = bootstrapHarness();
    harness.loadStartup.mockImplementationOnce(async () => {
      harness.order.push("load");
      return undefined;
    });

    await expect(startExternalMcpShadowWorkerBootstrap(
      {}, shadowConfig(), temporalConfig(), harness.baseActivities, {}, undefined, harness.seams
    )).resolves.toBeUndefined();

    expect(harness.order).toEqual(["definitions", "load"]);
    expect(harness.createResourceFactory).not.toHaveBeenCalled();
    expect(harness.startLifecycle).not.toHaveBeenCalled();
  });

  it("orders definitions, deployment, lazy RPC resource and lifecycle handoff", async () => {
    const harness = bootstrapHarness();
    const signal = new AbortController().signal;
    const onFailure = vi.fn();
    const env = { DIPOLE_AGENT_EXTERNAL_MCP_ENABLED: "true" };
    const shadow = shadowConfig();
    const temporal = temporalConfig();

    const lifecycle = await startExternalMcpShadowWorkerBootstrap(
      env,
      shadow,
      temporal,
      harness.baseActivities,
      { signal },
      onFailure,
      harness.seams
    );

    expect(lifecycle).toBe(harness.lifecycle);
    expect(harness.order).toEqual(["definitions", "load", "resource-factory", "resource", "lifecycle"]);
    expect(harness.loadStartup).toHaveBeenCalledWith(
      harness.definitions,
      env,
      harness.baseActivities,
      expect.any(Function),
      { signal }
    );
    expect(harness.createResourceFactory).toHaveBeenCalledWith(shadow);
    expect(harness.startLifecycle).toHaveBeenCalledWith(harness.startup, temporal, onFailure);
  });

  it("rejects cancellation before definitions without constructing dependencies", async () => {
    const harness = bootstrapHarness();
    const controller = new AbortController();
    controller.abort(new Error("cancelled before Shadow bootstrap"));

    await expect(startExternalMcpShadowWorkerBootstrap(
      {}, shadowConfig(), temporalConfig(), harness.baseActivities,
      { signal: controller.signal }, undefined, harness.seams
    )).rejects.toThrow(/cancelled before Shadow bootstrap/i);

    expect(harness.order).toEqual([]);
  });

  it("closes startup when cancellation wins before lifecycle handoff", async () => {
    const harness = bootstrapHarness();
    const controller = new AbortController();
    harness.loadStartup.mockImplementationOnce(async () => {
      harness.order.push("load");
      controller.abort(new Error("cancelled before lifecycle handoff"));
      return harness.startup;
    });

    await expect(startExternalMcpShadowWorkerBootstrap(
      {}, shadowConfig(), temporalConfig(), harness.baseActivities,
      { signal: controller.signal }, undefined, harness.seams
    )).rejects.toThrow(/cancelled before lifecycle handoff/i);

    expect(harness.order).toEqual(["definitions", "load", "startup-close"]);
    expect(harness.startLifecycle).not.toHaveBeenCalled();
    expect(harness.closeStartup).toHaveBeenCalledOnce();
  });

  it("reports a fixed cleanup failure when pre-handoff cancellation cannot close startup", async () => {
    const harness = bootstrapHarness({ closeError: new Error("sensitive RPC target") });
    const controller = new AbortController();
    harness.loadStartup.mockImplementationOnce(async () => {
      harness.order.push("load");
      controller.abort(new Error("sensitive cancellation"));
      return harness.startup;
    });

    await expect(startExternalMcpShadowWorkerBootstrap(
      {}, shadowConfig(), temporalConfig(), harness.baseActivities,
      { signal: controller.signal }, undefined, harness.seams
    )).rejects.toThrow(/^External MCP Shadow Worker bootstrap cleanup failed$/);
  });

  it("keeps deployment failures low-sensitive before ownership handoff", async () => {
    const harness = bootstrapHarness();
    harness.loadStartup.mockRejectedValueOnce(new Error("sensitive manifest path"));

    await expect(startExternalMcpShadowWorkerBootstrap(
      {}, shadowConfig(), temporalConfig(), harness.baseActivities, {}, undefined, harness.seams
    )).rejects.toThrow(/^External MCP Shadow Worker bootstrap is unavailable$/);

    expect(harness.closeStartup).not.toHaveBeenCalled();
    expect(harness.startLifecycle).not.toHaveBeenCalled();
  });

  it("does not reclaim startup after lifecycle accepts ownership and fails", async () => {
    const harness = bootstrapHarness();
    harness.startLifecycle.mockImplementationOnce(async startup => {
      harness.order.push("lifecycle");
      await startup!.close();
      throw new Error("External MCP Temporal Worker lifecycle startup failed");
    });

    await expect(startExternalMcpShadowWorkerBootstrap(
      {}, shadowConfig(), temporalConfig(), harness.baseActivities, {}, undefined, harness.seams
    )).rejects.toThrow(/^External MCP Temporal Worker lifecycle startup failed$/);

    expect(harness.closeStartup).toHaveBeenCalledOnce();
  });
});

function bootstrapHarness(options: { readonly closeError?: Error } = {}) {
  const order: string[] = [];
  const definitions = {} as ExternalMcpCapabilityDefinitionRegistry;
  const deployment = { deployment: true } as unknown as ExternalMcpDeploymentPlan;
  const worker = { worker: true } as unknown as ExternalMcpTemporalWorkerComposition;
  const closeStartup = vi.fn(async () => {
    order.push("startup-close");
    if (options.closeError !== undefined) throw options.closeError;
  });
  const startup = { deployment, worker, close: closeStartup } satisfies ExternalMcpTemporalWorkerStartupPlan;
  const lifecycle = { deployment, worker, stop: vi.fn() } satisfies ExternalMcpTemporalWorkerLifecycle;
  const resource = {
    dependencies: { core: {}, artifacts: {} }, close: vi.fn()
  } as unknown as ExternalMcpTemporalWorkerResource;
  const createDefinitions = vi.fn<ExternalMcpShadowWorkerBootstrapSeams["createDefinitions"]>(() => {
    order.push("definitions");
    return definitions;
  });
  const createResource = vi.fn(async () => {
    order.push("resource");
    return resource;
  });
  const createResourceFactory = vi.fn<ExternalMcpShadowWorkerBootstrapSeams["createResourceFactory"]>(() => {
    order.push("resource-factory");
    return createResource;
  });
  const loadStartup = vi.fn<ExternalMcpShadowWorkerBootstrapSeams["loadStartup"]>(
    async (_definitions, _env, _activities, resourceFactory, startupOptions) => {
      order.push("load");
      await resourceFactory(deployment, startupOptions?.signal ?? new AbortController().signal);
      return startup;
    }
  );
  const startLifecycle = vi.fn<ExternalMcpShadowWorkerBootstrapSeams["startLifecycle"]>(async () => {
    order.push("lifecycle");
    return lifecycle;
  });
  const seams = { createDefinitions, createResourceFactory, loadStartup, startLifecycle };
  return {
    order,
    definitions,
    deployment,
    worker,
    startup,
    lifecycle,
    closeStartup,
    createResourceFactory,
    loadStartup,
    startLifecycle,
    seams,
    baseActivities: {} as AgentTaskWorkerActivities
  };
}

function shadowConfig() {
  return loadShadowRuntimeConfig({
    DIPOLE_AGENT_TENANT_ID: "dipole",
    DIPOLE_AGENT_CAPABILITY_RPC_ENABLED: "true",
    DIPOLE_AGENT_CAPABILITY_RPC_TARGET: "127.0.0.1:50061",
    DIPOLE_INTERNAL_RPC_SHARED_SECRET: "test-only-shared-secret"
  });
}

function temporalConfig() {
  return loadTemporalRuntimeConfig({ DIPOLE_AGENT_TEMPORAL_ENABLED: "true" });
}
