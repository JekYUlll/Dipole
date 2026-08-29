import { describe, expect, it, vi } from "vitest";

import {
  createTemporalMcpMultiRouteRuntime,
  type TemporalMcpDispatchRuntimeFactory,
  type TemporalMcpMultiRoutePlan
} from "./mcp-multi-route-runtime.js";
import {
  temporalMcpDispatchRouteBinding,
  type TemporalMcpDispatchActivityInput,
  type TemporalMcpDispatchRoute
} from "./mcp-dispatch-activity.js";
import type { TemporalMcpDispatchRuntimeDependencies } from "./mcp-dispatch-runtime.js";

const calendarRoute: TemporalMcpDispatchRoute = {
  routeId: "calendar-event-read",
  routeVersion: 1,
  capabilityId: "calendar.event.read",
  workflowStep: 3,
  ordinal: 1,
  deploymentBindingSha256: "a".repeat(64)
};
const issueRoute: TemporalMcpDispatchRoute = {
  routeId: "github-issue-read",
  routeVersion: 2,
  capabilityId: "github.issue.read",
  workflowStep: 4,
  ordinal: 1,
  deploymentBindingSha256: "b".repeat(64)
};

describe("Temporal MCP multi-route Runtime", () => {
  it("constructs every route once and delegates begin by host-loaded route ID", async () => {
    const harness = runtimeHarness([calendarRoute, issueRoute]);

    const runtime = createTemporalMcpMultiRouteRuntime(harness.plan, harness.dependencies, harness.factory);
    const result = await runtime.activities.executeMcpDispatch(beginInput(issueRoute));

    expect(result).toEqual({ kind: "complete", output: { routeId: issueRoute.routeId } });
    expect(harness.constructed).toEqual([calendarRoute.routeId, issueRoute.routeId]);
    expect(harness.executed).toEqual([issueRoute.routeId]);
    expect(runtime.routeBindings).toEqual([
      temporalMcpDispatchRouteBinding(calendarRoute),
      temporalMcpDispatchRouteBinding(issueRoute)
    ]);
    expect(harness.factory.mock.calls[0]?.[1]).toMatchObject({
      routes: harness.plan.routeRegistry,
      externalMcp: harness.plan.workerExternalMcp,
      core: harness.dependencies.core,
      artifacts: harness.dependencies.artifacts
    });
  });

  it("selects durable resume routing only from its persisted checkpoint", async () => {
    const harness = runtimeHarness([calendarRoute, issueRoute]);
    const runtime = createTemporalMcpMultiRouteRuntime(harness.plan, harness.dependencies, harness.factory);

    await runtime.activities.executeMcpDispatch({
      kind: "resume",
      checkpoint: {
        schemaVersion: "dipole.mcp.temporal-dispatch-checkpoint.v1",
        routeId: calendarRoute.routeId
      },
      resume: { kind: "input", requestId: "INPUT-1", value: { choice: "yes" } }
    } as unknown as TemporalMcpDispatchActivityInput);

    expect(harness.executed).toEqual([calendarRoute.routeId]);
  });

  it("rejects malformed or unknown routing before any route Runtime executes", async () => {
    const harness = runtimeHarness([calendarRoute, issueRoute]);
    const runtime = createTemporalMcpMultiRouteRuntime(harness.plan, harness.dependencies, harness.factory);

    await expect(runtime.activities.executeMcpDispatch({
      ...beginInput(calendarRoute), routeId: "unknown-route"
    })).rejects.toThrow(/route is unavailable/i);
    await expect(runtime.activities.executeMcpDispatch({
      kind: "resume", checkpoint: {}, resume: { kind: "input", requestId: "INPUT-1", value: {} }
    } as unknown as TemporalMcpDispatchActivityInput)).rejects.toThrow(/routing input is invalid/i);
    expect(harness.executed).toEqual([]);
  });

  it("fails construction on empty or duplicate route identity", () => {
    const empty = runtimeHarness([]);
    expect(() => createTemporalMcpMultiRouteRuntime(empty.plan, empty.dependencies, empty.factory))
      .toThrow(/routes are unavailable/i);

    const duplicated = runtimeHarness([calendarRoute, { ...issueRoute, routeId: calendarRoute.routeId }]);
    expect(() => createTemporalMcpMultiRouteRuntime(duplicated.plan, duplicated.dependencies, duplicated.factory))
      .toThrow(/route ID is duplicated/i);
    expect(duplicated.executed).toEqual([]);
  });

  it("preserves route-local binding failures and cancellation reasons", async () => {
    const drifted = runtimeHarness([calendarRoute]);
    const driftedRuntime = createTemporalMcpMultiRouteRuntime(
      drifted.plan, drifted.dependencies, drifted.factory
    );
    await expect(driftedRuntime.activities.executeMcpDispatch({
      ...beginInput(calendarRoute), routeManifestSha256: "f".repeat(64)
    })).rejects.toThrow(/route-local binding is invalid/i);
    expect(drifted.executed).toEqual([calendarRoute.routeId]);

    const controller = new AbortController();
    controller.abort(new Error("cancelled by Temporal Activity"));
    const harness = runtimeHarness([calendarRoute], controller.signal);
    const runtime = createTemporalMcpMultiRouteRuntime(harness.plan, harness.dependencies, harness.factory);

    await expect(runtime.activities.executeMcpDispatch(beginInput(calendarRoute)))
      .rejects.toThrow(/cancelled by Temporal Activity/i);
    expect(harness.executed).toEqual([calendarRoute.routeId]);
  });
});

function runtimeHarness(routes: readonly TemporalMcpDispatchRoute[], abortedSignal?: AbortSignal) {
  const constructed: string[] = [];
  const executed: string[] = [];
  const factory = vi.fn<TemporalMcpDispatchRuntimeFactory>((route) => {
    constructed.push(route.routeId);
    return {
      routeBinding: temporalMcpDispatchRouteBinding(route),
      activities: {
        executeMcpDispatch: async input => {
          executed.push(route.routeId);
          abortedSignal?.throwIfAborted();
          if (input.kind === "begin" &&
              input.routeManifestSha256 !== temporalMcpDispatchRouteBinding(route).routeManifestSha256) {
            throw new Error("route-local binding is invalid");
          }
          return { kind: "complete", output: { routeId: route.routeId } };
        }
      }
    };
  });
  const routeRegistry = { plan: "routes" };
  const workerExternalMcp = { plan: "external-mcp" };
  const plan = {
    routes,
    routeRegistry,
    workerExternalMcp
  } as unknown as TemporalMcpMultiRoutePlan;
  const dependencies = {
    core: { dependency: "core" },
    artifacts: { dependency: "artifacts" }
  } as unknown as Omit<TemporalMcpDispatchRuntimeDependencies, "routes" | "externalMcp">;
  return { plan, dependencies, factory, constructed, executed };
}

function beginInput(
  route: TemporalMcpDispatchRoute
): Extract<TemporalMcpDispatchActivityInput, { kind: "begin" }> {
  return {
    kind: "begin",
    ...temporalMcpDispatchRouteBinding(route),
    taskId: "TASK-1",
    runId: "RUN-1",
    principalUserId: "U100",
    arguments: {}
  };
}
