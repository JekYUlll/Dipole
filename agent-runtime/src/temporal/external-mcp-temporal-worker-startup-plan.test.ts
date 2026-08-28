import { describe, expect, it, vi } from "vitest";

import { ExternalMcpCapabilityDefinitionRegistry } from "../mcp/external-mcp-deployment-route-manifest.js";
import type { ExternalMcpDeploymentPlan } from "../mcp/external-mcp-deployment-composition.js";
import { foundationAgentTaskActivities } from "./agent-task-activities.js";
import type { ExternalMcpTemporalWorkerComposition } from "./external-mcp-temporal-worker-composition.js";
import {
  loadExternalMcpTemporalWorkerStartupPlan,
  type ExternalMcpTemporalWorkerResource,
  type ExternalMcpTemporalWorkerStartupSeams
} from "./external-mcp-temporal-worker-startup-plan.js";

describe("external MCP Temporal Worker startup plan", () => {
  it("loads disabled deployment without creating resources or composition", async () => {
    const harness = startupHarness(undefined);

    await expect(loadExternalMcpTemporalWorkerStartupPlan(
      harness.definitions,
      {},
      foundationAgentTaskActivities,
      harness.createResource,
      {},
      harness.seams
    )).resolves.toBeUndefined();

    expect(harness.loadDeployment).toHaveBeenCalledOnce();
    expect(harness.createResource).not.toHaveBeenCalled();
    expect(harness.compose).not.toHaveBeenCalled();
  });

  it("constructs in load-resource-compose order and owns idempotent cleanup", async () => {
    const order: string[] = [];
    const harness = startupHarness(deployment(), order);

    const startup = await loadExternalMcpTemporalWorkerStartupPlan(
      harness.definitions,
      { DIPOLE_AGENT_EXTERNAL_MCP_ENABLED: "true" },
      foundationAgentTaskActivities,
      harness.createResource,
      { maximumIoManifestBytes: 4096 },
      harness.seams
    );
    if (startup === undefined) throw new Error("expected startup plan");

    expect(order).toEqual(["load", "validate", "resource", "compose"]);
    expect(startup.deployment).toBe(harness.deployment);
    expect(startup.worker).toBe(harness.worker);
    expect(harness.compose.mock.calls[0]?.[2]()).toBe(harness.resource.dependencies);
    const loaderOptions = harness.loadDeployment.mock.calls[0]?.[2];
    expect(loaderOptions).toMatchObject({ maximumIoManifestBytes: 4096, signal: expect.any(AbortSignal) });

    await startup.close();
    await startup.close();
    expect(harness.closeResource).toHaveBeenCalledOnce();
  });

  it("composes with the post-resource Worker Activity snapshot", async () => {
    const harness = startupHarness(deployment());
    const workerActivities = {
      ...foundationAgentTaskActivities,
      admitAgentTask: vi.fn(foundationAgentTaskActivities.admitAgentTask)
    };
    (harness.resource as unknown as { workerActivities?: typeof workerActivities }).workerActivities = workerActivities;

    await loadExternalMcpTemporalWorkerStartupPlan(
      harness.definitions,
      {},
      foundationAgentTaskActivities,
      harness.createResource,
      {},
      harness.seams
    );

    expect(harness.validateCompositionPlan).toHaveBeenCalledWith(harness.deployment, foundationAgentTaskActivities);
    expect(harness.compose).toHaveBeenCalledWith(harness.deployment, workerActivities, expect.any(Function));
  });

  it("propagates cancellation before loading or between load and resource creation", async () => {
    const before = new AbortController();
    before.abort(new Error("cancelled before deployment load"));
    const untouched = startupHarness(deployment());
    await expect(loadExternalMcpTemporalWorkerStartupPlan(
      untouched.definitions,
      {},
      foundationAgentTaskActivities,
      untouched.createResource,
      { signal: before.signal },
      untouched.seams
    )).rejects.toThrow(/cancelled before deployment load/i);
    expect(untouched.loadDeployment).not.toHaveBeenCalled();

    const between = new AbortController();
    const loaded = startupHarness(deployment());
    loaded.loadDeployment.mockImplementationOnce(async () => {
      between.abort(new Error("cancelled after deployment load"));
      return loaded.deployment;
    });
    await expect(loadExternalMcpTemporalWorkerStartupPlan(
      loaded.definitions,
      {},
      foundationAgentTaskActivities,
      loaded.createResource,
      { signal: between.signal },
      loaded.seams
    )).rejects.toThrow(/cancelled after deployment load/i);
    expect(loaded.createResource).not.toHaveBeenCalled();
  });

  it("closes a created resource when cancellation wins before composition", async () => {
    const controller = new AbortController();
    const harness = startupHarness(deployment());
    harness.createResource.mockImplementationOnce(async () => {
      controller.abort(new Error("cancelled after resource creation"));
      return harness.resource;
    });

    await expect(loadExternalMcpTemporalWorkerStartupPlan(
      harness.definitions,
      {},
      foundationAgentTaskActivities,
      harness.createResource,
      { signal: controller.signal },
      harness.seams
    )).rejects.toThrow(/cancelled after resource creation/i);
    expect(harness.closeResource).toHaveBeenCalledOnce();
    expect(harness.compose).not.toHaveBeenCalled();
  });

  it("does not create a resource when static composition validation fails", async () => {
    const harness = startupHarness(deployment());
    harness.validateCompositionPlan.mockImplementationOnce(() => {
      throw new Error("invalid route authority");
    });

    await expect(loadExternalMcpTemporalWorkerStartupPlan(
      harness.definitions,
      {},
      foundationAgentTaskActivities,
      harness.createResource,
      {},
      harness.seams
    )).rejects.toThrow(/^External MCP Temporal Worker startup plan is unavailable$/);
    expect(harness.createResource).not.toHaveBeenCalled();
    expect(harness.compose).not.toHaveBeenCalled();
  });

  it("rolls back a resource after composition failure with a low-sensitive error", async () => {
    const harness = startupHarness(deployment());
    harness.compose.mockImplementationOnce(() => {
      throw new Error("secret internal composition detail");
    });

    await expect(loadExternalMcpTemporalWorkerStartupPlan(
      harness.definitions,
      {},
      foundationAgentTaskActivities,
      harness.createResource,
      {},
      harness.seams
    )).rejects.toThrow(/^External MCP Temporal Worker startup plan is unavailable$/);
    expect(harness.closeResource).toHaveBeenCalledOnce();
  });

  it("memoizes a failing explicit close without invoking the resource twice", async () => {
    const harness = startupHarness(deployment());
    harness.closeResource.mockRejectedValueOnce(new Error("resource close failed"));
    const startup = await loadExternalMcpTemporalWorkerStartupPlan(
      harness.definitions,
      {},
      foundationAgentTaskActivities,
      harness.createResource,
      {},
      harness.seams
    );
    if (startup === undefined) throw new Error("expected startup plan");

    await expect(startup.close()).rejects.toThrow(/^External MCP Temporal Worker resource cleanup failed$/);
    await expect(startup.close()).rejects.toThrow(/^External MCP Temporal Worker resource cleanup failed$/);
    expect(harness.closeResource).toHaveBeenCalledOnce();
  });

  it("reports rollback cleanup failure without exposing construction or resource details", async () => {
    const harness = startupHarness(deployment());
    harness.compose.mockImplementationOnce(() => {
      throw new Error("secret composition failure");
    });
    harness.closeResource.mockRejectedValueOnce(new Error("sensitive RPC target"));

    await expect(loadExternalMcpTemporalWorkerStartupPlan(
      harness.definitions,
      {},
      foundationAgentTaskActivities,
      harness.createResource,
      {},
      harness.seams
    )).rejects.toThrow(/^External MCP Temporal Worker resource cleanup failed$/);
    expect(harness.closeResource).toHaveBeenCalledOnce();
  });
});

function startupHarness(plan: ExternalMcpDeploymentPlan | undefined, order: string[] = []) {
  const definitions = new ExternalMcpCapabilityDefinitionRegistry();
  const worker = { worker: true } as unknown as ExternalMcpTemporalWorkerComposition;
  const closeResource = vi.fn(async () => undefined);
  const resource = {
    dependencies: { core: { core: true }, artifacts: { artifacts: true } },
    close: closeResource
  } as unknown as ExternalMcpTemporalWorkerResource;
  const loadDeployment = vi.fn<ExternalMcpTemporalWorkerStartupSeams["loadDeployment"]>(async () => {
    order.push("load");
    return plan;
  });
  const createResource = vi.fn(async () => {
    order.push("resource");
    return resource;
  });
  const validateCompositionPlan = vi.fn<ExternalMcpTemporalWorkerStartupSeams["validateCompositionPlan"]>(() => {
    order.push("validate");
  });
  const compose = vi.fn<ExternalMcpTemporalWorkerStartupSeams["compose"]>((_plan, _activities, resolveDependencies) => {
    order.push("compose");
    resolveDependencies();
    return worker;
  });
  const seams: ExternalMcpTemporalWorkerStartupSeams = {
    loadDeployment,
    validateCompositionPlan,
    compose
  };
  return {
    definitions,
    deployment: plan,
    worker,
    resource,
    closeResource,
    loadDeployment,
    createResource,
    validateCompositionPlan,
    compose,
    seams
  };
}

function deployment(): ExternalMcpDeploymentPlan {
  return { deployment: true } as unknown as ExternalMcpDeploymentPlan;
}
