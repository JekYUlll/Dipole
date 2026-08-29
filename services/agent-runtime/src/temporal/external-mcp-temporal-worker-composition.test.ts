import { z } from "zod";
import { describe, expect, it, vi } from "vitest";

import { foundationAgentTaskActivities } from "./agent-task-activities.js";
import {
  createExternalMcpTemporalWorkerComposition,
  type ExternalMcpTemporalWorkerCompositionPlan,
  type TemporalMcpMultiRouteRuntimeFactory
} from "./external-mcp-temporal-worker-composition.js";
import { temporalMcpDispatchRouteBinding } from "./mcp-dispatch-activity.js";
import { ExternalMcpCapabilityRouteRegistry } from "../mcp/mcp-invocation-producer.js";
import type { TemporalMcpDispatchRuntimeCore } from "./mcp-dispatch-runtime.js";
import { externalMcpReadinessBindingSha256 } from "../mcp/external-mcp-readiness-evidence.js";

const calendarRoute = {
  routeId: "calendar-event-read",
  routeVersion: 3,
  capabilityId: "calendar.event.read",
  workflowStep: 4,
  ordinal: 1,
  deploymentBindingSha256: "a".repeat(64)
};

describe("external MCP Temporal Worker composition", () => {
  it("does not resolve ports or construct a Runtime while the deployment plan is disabled", () => {
    const resolveDependencies = vi.fn();
    const createRuntime = vi.fn<TemporalMcpMultiRouteRuntimeFactory>();

    expect(createExternalMcpTemporalWorkerComposition(
      undefined,
      foundationAgentTaskActivities,
      resolveDependencies,
      createRuntime
    )).toBeUndefined();
    expect(resolveDependencies).not.toHaveBeenCalled();
    expect(createRuntime).not.toHaveBeenCalled();
  });

  it("returns one exact Activity registration and matching host Workflow catalog", async () => {
    const plan = deploymentPlan([calendarRoute]);
    const dependencies = dependencyPorts();
    const resolveDependencies = vi.fn(() => dependencies.value);
    const executeMcpDispatch = vi.fn(async () => ({ kind: "complete" as const, output: { ok: true } }));
    const createRuntime = vi.fn<TemporalMcpMultiRouteRuntimeFactory>(() => ({
      routeBindings: [temporalMcpDispatchRouteBinding(calendarRoute)],
      activities: { executeMcpDispatch }
    }));

    const composition = createExternalMcpTemporalWorkerComposition(
      plan,
      foundationAgentTaskActivities,
      resolveDependencies,
      createRuntime
    );
    if (composition === undefined) throw new Error("expected composition");

    expect(resolveDependencies).toHaveBeenCalledOnce();
    expect(createRuntime).toHaveBeenCalledWith(plan, dependencies.value);
    expect(composition.runtimeBindingSha256).toBe(plan.runtimeBindingSha256);
    expect(composition.subscriptionRoutes).toEqual([]);
    expect(Object.isFrozen(composition.subscriptionRoutes)).toBe(true);
    expect(composition.routeBindings).toEqual([temporalMcpDispatchRouteBinding(calendarRoute)]);
    expect(composition.workflowExecutions.create(calendarRoute.routeId, { eventId: "EV-1" })).toEqual({
      kind: "external_mcp_v1",
      ...temporalMcpDispatchRouteBinding(calendarRoute),
      arguments: { eventId: "EV-1" }
    });
    expect(composition.activities.executeAgentTaskStep)
      .toBe(foundationAgentTaskActivities.executeAgentTaskStep);
    await expect(composition.activities.executeMcpDispatch({} as never))
      .resolves.toEqual({ kind: "complete", output: { ok: true } });
  });

  it("rejects invalid authority and Activity collisions before resolving dependency ports", () => {
    const resolveDependencies = vi.fn(() => dependencyPorts().value);
    const createRuntime = vi.fn<TemporalMcpMultiRouteRuntimeFactory>();

    expect(() => createExternalMcpTemporalWorkerComposition(
      { ...deploymentPlan([calendarRoute]), runtimeBindingSha256: "invalid" },
      foundationAgentTaskActivities,
      resolveDependencies,
      createRuntime
    )).toThrow(/Runtime binding is invalid/i);
    expect(() => createExternalMcpTemporalWorkerComposition(
      { ...deploymentPlan([calendarRoute]), runtimeBindingSha256: "e".repeat(64) },
      foundationAgentTaskActivities,
      resolveDependencies,
      createRuntime
    )).toThrow(/Runtime binding is conflicting/i);
    expect(() => createExternalMcpTemporalWorkerComposition(
      deploymentPlan([calendarRoute, { ...calendarRoute }]),
      foundationAgentTaskActivities,
      resolveDependencies,
      createRuntime
    )).toThrow(/route ID is duplicated/i);
    expect(() => createExternalMcpTemporalWorkerComposition(
      deploymentPlan([calendarRoute]),
      { ...foundationAgentTaskActivities, executeMcpDispatch: vi.fn() } as never,
      resolveDependencies,
      createRuntime
    )).toThrow(/Activity collision/i);
    expect(() => createExternalMcpTemporalWorkerComposition(
      deploymentPlan([{ ...calendarRoute, capabilityId: "calendar.event.missing" }]),
      foundationAgentTaskActivities,
      resolveDependencies,
      createRuntime
    )).toThrow(/route is unavailable/i);
    expect(() => createExternalMcpTemporalWorkerComposition(
      {
        ...deploymentPlan([calendarRoute]),
        subscriptionRoutes: [{
          definitionId: "DEF-1", definitionVersion: 1, routeId: "missing-route", resolveArguments: () => ({})
        }]
      },
      foundationAgentTaskActivities,
      resolveDependencies,
      createRuntime
    )).toThrow(/conflict with the Worker catalog/i);
    expect(resolveDependencies).not.toHaveBeenCalled();
    expect(createRuntime).not.toHaveBeenCalled();
  });

  it("rejects a Runtime factory that returns bindings outside the deployment plan", () => {
    const plan = deploymentPlan([calendarRoute]);
    const resolveDependencies = vi.fn(() => dependencyPorts().value);
    const createRuntime = vi.fn<TemporalMcpMultiRouteRuntimeFactory>(() => ({
      routeBindings: [{
        ...temporalMcpDispatchRouteBinding(calendarRoute),
        routeManifestSha256: "d".repeat(64)
      }],
      activities: { executeMcpDispatch: vi.fn() }
    }));

    expect(() => createExternalMcpTemporalWorkerComposition(
      plan,
      foundationAgentTaskActivities,
      resolveDependencies,
      createRuntime
    )).toThrow(/route bindings are conflicting/i);
    expect(resolveDependencies).toHaveBeenCalledOnce();
  });

  it("constructs the real multi-route Runtime without touching Core, Artifact, or network ports", () => {
    const plan = deploymentPlan([calendarRoute]);
    const dependencies = dependencyPorts();

    const composition = createExternalMcpTemporalWorkerComposition(
      plan,
      foundationAgentTaskActivities,
      () => dependencies.value
    );

    expect(composition?.routeBindings).toEqual([temporalMcpDispatchRouteBinding(calendarRoute)]);
    expect(dependencies.coreCalls).not.toHaveBeenCalled();
    expect(dependencies.createArtifact).not.toHaveBeenCalled();
    expect(plan.workerExternalMcp.registry.connect).not.toHaveBeenCalled();
  });
});

function deploymentPlan(routes: readonly typeof calendarRoute[]): ExternalMcpTemporalWorkerCompositionPlan {
  const registry = new ExternalMcpCapabilityRouteRegistry();
  registry.register({
    descriptor: { id: "calendar.event.read", risk: "read", requiredPermission: "calendar.read" },
    inputSchema: z.object({ eventId: z.string() }).strict(),
    profileId: "calendar-prod",
    serverId: "calendar.example",
    toolName: "calendar.read_event",
    egressPolicy: { allowedArgumentNames: ["eventId"], maximumBytes: 1024 },
    resolveResource: input => ({ resourceType: "calendar", resourceId: input.eventId, action: "read" })
  });
  const connect = vi.fn();
  const config = {
    enabled: true,
    profiles: [{
      profileId: "calendar-prod",
      tenantId: "dipole",
      serverId: "calendar.example",
      endpoint: "https://calendar.example/v1",
      credentialRef: "CRED-0123456789ABCDEF",
      credentialVersion: 1,
      allowedHosts: ["calendar.example"],
      allowedPorts: [443],
      dnsResolution: "public_only",
      tlsServerName: "calendar.example",
      caBundleRef: "CA-0123456789ABCDEF",
      allowedTools: ["calendar.read_event"]
    }]
  } as const;
  const io = {
    credentialCatalogPath: "/run/dipole/catalog.json",
    secretProvider: {
      providerId: "local-aes-gcm",
      keys: { "KEY-0123456789ABCDEF": "/run/dipole/key.bin" },
      secrets: {
        "SECRET-0123456789ABCDEF": {
          keyRef: "KEY-0123456789ABCDEF",
          path: "/run/dipole/secret.bin"
        }
      }
    },
    caBundles: { "CA-0123456789ABCDEF": "/run/dipole/ca.pem" }
  };
  const readinessBindingOptions = { expectedOwnerUid: 1000, trustedTransportBuilder: true } as const;
  return {
    routes,
    routeRegistry: registry,
    workerExternalMcp: {
      config,
      io,
      registry: {
        describe: () => ({
          profileId: "calendar-prod",
          tenantId: "dipole",
          serverId: "calendar.example",
          endpoint: "https://calendar.example/v1",
          credentialRef: "CRED-0123456789ABCDEF",
          credentialVersion: 1,
          allowedHosts: ["calendar.example"],
          allowedPorts: [443],
          dnsResolution: "public_only",
          tlsServerName: "calendar.example",
          caBundleRef: "CA-0123456789ABCDEF",
          allowedTools: ["calendar.read_event"]
        }),
        connect
      },
      readinessBindingOptions
    },
    runtimeBindingSha256: externalMcpReadinessBindingSha256(config, io, readinessBindingOptions)
  };
}

function dependencyPorts() {
  const coreCalls = vi.fn();
  const core = new Proxy({}, {
    get: () => coreCalls
  }) as TemporalMcpDispatchRuntimeCore;
  const createArtifact = vi.fn();
  return {
    value: {
      core,
      artifacts: { createArtifact }
    },
    coreCalls,
    createArtifact
  };
}
