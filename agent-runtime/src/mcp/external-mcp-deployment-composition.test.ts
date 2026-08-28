import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it, vi } from "vitest";
import { z } from "zod";

import {
  ExternalMcpCapabilityDefinitionRegistry
} from "./external-mcp-deployment-route-manifest.js";
import {
  loadExternalMcpDeploymentPlan
} from "./external-mcp-deployment-composition.js";
import {
  externalMcpReadinessBindingSha256
} from "./external-mcp-readiness-evidence.js";

const directories: string[] = [];

afterEach(async () => {
  await Promise.all(directories.splice(0).map(path => rm(path, { recursive: true, force: true })));
});

describe("external MCP deployment composition", () => {
  it("keeps disabled deployment side-effect free without reading residual manifests", async () => {
    const env = { DIPOLE_AGENT_EXTERNAL_MCP_ENABLED: "false" } as NodeJS.ProcessEnv;
    for (const name of [
      "DIPOLE_AGENT_EXTERNAL_MCP_PROFILES",
      "DIPOLE_AGENT_EXTERNAL_MCP_IO_MANIFEST",
      "DIPOLE_AGENT_EXTERNAL_MCP_ROUTE_MANIFEST"
    ]) {
      Object.defineProperty(env, name, {
        get: () => { throw new Error(`disabled deployment read ${name}`); }
      });
    }

    await expect(loadExternalMcpDeploymentPlan(definitions(), env)).resolves.toBeUndefined();
  });

  it("loads one exact side-effect-free plan for readiness collection and gated Worker egress", async () => {
    const fixture = await deploymentFixture();
    const plan = await loadExternalMcpDeploymentPlan(definitions(), fixture.env, {
      expectedOwnerUid: process.getuid!(),
      maximumReadinessCollectionMs: 120_000,
      now: () => new Date("2026-08-28T16:00:00.000Z")
    });
    if (plan === undefined) throw new Error("expected enabled plan");

    expect(Object.keys(plan).sort()).toEqual([
      "config", "productionIo", "routeRegistry", "routes", "runtimeBindingSha256", "workerExternalMcp"
    ]);
    expect(plan.routes).toEqual([{
      routeId: "calendar-event-read", routeVersion: 1, capabilityId: "calendar.event.read",
      workflowStep: 4, ordinal: 1, deploymentBindingSha256: expect.stringMatching(/^[a-f0-9]{64}$/)
    }]);
    expect(plan.productionIo.registry.describe("calendar-prod", "dipole")).toMatchObject({
      serverId: "calendar.example", allowedTools: ["calendar.read_event"]
    });
    expect(plan.workerExternalMcp.registry).toBe(plan.productionIo.registry);
    expect(plan.workerExternalMcp.config).toBe(plan.config);
    expect(plan.runtimeBindingSha256).toBe(externalMcpReadinessBindingSha256(
      plan.config,
      plan.workerExternalMcp.io,
      plan.workerExternalMcp.readinessBindingOptions
    ));
    expect(plan.routeRegistry.workerEgressPolicies("calendar.event.read")).toEqual({
      "calendar-prod": {
        "calendar.read_event": { allowedArgumentNames: ["calendarId", "eventId"], maximumBytes: 1024 }
      }
    });
  });

  it("does not touch referenced Catalog, key, secret or CA files during composition", async () => {
    const fixture = await deploymentFixture("missing-runtime-files");
    await expect(loadExternalMcpDeploymentPlan(definitions(), fixture.env, owner()))
      .resolves.toMatchObject({ runtimeBindingSha256: expect.stringMatching(/^[a-f0-9]{64}$/) });
  });

  it("fails as one low-sensitive unit when either manifest or cross-binding drifts", async () => {
    const missingIo = await deploymentFixture();
    delete missingIo.env.DIPOLE_AGENT_EXTERNAL_MCP_IO_MANIFEST;
    await expect(loadExternalMcpDeploymentPlan(definitions(), missingIo.env, owner()))
      .rejects.toThrow("External MCP deployment plan is unavailable");

    const driftedRoute = await deploymentFixture();
    await writeFile(driftedRoute.routePath, JSON.stringify(routeManifest("calendar.delete_event")), { mode: 0o600 });
    const error = await loadExternalMcpDeploymentPlan(definitions(), driftedRoute.env, owner())
      .catch((caught: unknown) => caught);
    expect(String(error)).toBe("Error: External MCP deployment plan is unavailable");
    expect(String(error)).not.toMatch(/delete_event|calendar\.example|\/run\/dipole/i);
  });

  it("propagates pre-load and between-manifest cancellation without returning a partial plan", async () => {
    const before = new AbortController();
    before.abort(new Error("cancelled before deployment plan"));
    const untouched = {} as NodeJS.ProcessEnv;
    Object.defineProperty(untouched, "DIPOLE_AGENT_EXTERNAL_MCP_ENABLED", {
      get: vi.fn(() => "true")
    });
    await expect(loadExternalMcpDeploymentPlan(definitions(), untouched, { signal: before.signal }))
      .rejects.toThrow(/cancelled before deployment plan/i);
    expect(Object.getOwnPropertyDescriptor(untouched, "DIPOLE_AGENT_EXTERNAL_MCP_ENABLED")?.get)
      .not.toHaveBeenCalled();

    const between = new AbortController();
    const fixture = await deploymentFixture();
    Object.defineProperty(fixture.env, "DIPOLE_AGENT_EXTERNAL_MCP_ROUTE_MANIFEST", {
      get: () => {
        between.abort(new Error("cancelled between deployment manifests"));
        return fixture.routePath;
      }
    });
    await expect(loadExternalMcpDeploymentPlan(definitions(), fixture.env, {
      ...owner(),
      signal: between.signal
    })).rejects.toThrow(/cancelled between deployment manifests/i);
  });
});

function definitions() {
  const registry = new ExternalMcpCapabilityDefinitionRegistry();
  registry.register({
    descriptor: { id: "calendar.event.read", risk: "read", requiredPermission: "calendar.read" },
    inputSchema: z.object({ calendarId: z.string(), eventId: z.string() }).strict(),
    egressCeiling: { allowedArgumentNames: ["calendarId", "eventId"], maximumBytes: 4096 },
    resolveResource: input => ({ resourceType: "calendar", resourceId: input.calendarId, action: "read" })
  });
  return registry;
}

async function deploymentFixture(runtimeDirectory = "runtime"): Promise<{
  env: NodeJS.ProcessEnv;
  routePath: string;
}> {
  const directory = await mkdtemp(join(tmpdir(), "dipole-mcp-deployment-plan-"));
  directories.push(directory);
  const ioPath = join(directory, "io.json");
  const routePath = join(directory, "routes.json");
  const runtimeRoot = join(directory, runtimeDirectory);
  await writeFile(ioPath, JSON.stringify(ioManifest(runtimeRoot)), { mode: 0o600 });
  await writeFile(routePath, JSON.stringify(routeManifest()), { mode: 0o600 });
  return {
    routePath,
    env: {
      DIPOLE_AGENT_EXTERNAL_MCP_ENABLED: "true",
      DIPOLE_AGENT_EXTERNAL_MCP_PROFILES: JSON.stringify([profile()]),
      DIPOLE_AGENT_EXTERNAL_MCP_IO_MANIFEST: ioPath,
      DIPOLE_AGENT_EXTERNAL_MCP_ROUTE_MANIFEST: routePath
    }
  };
}

function profile() {
  return {
    schema_version: "dipole.agent.external-mcp-profile.v1",
    profile_id: "calendar-prod", tenant_id: "dipole", server_id: "calendar.example",
    endpoint: "https://calendar.example/v1", credential: { ref: "CRED-0123456789ABCDEF", version: 1 },
    network_policy: {
      allowed_hosts: ["calendar.example"], allowed_ports: [443], dns_resolution: "public_only",
      tls_server_name: "calendar.example", ca_bundle_ref: "CA-0123456789ABCDEF"
    },
    allowed_tools: ["calendar.read_event"]
  };
}

function ioManifest(root: string) {
  return {
    schema_version: "dipole.agent.external-mcp-production-io.v1",
    credential_catalog: { path: join(root, "catalog.json"), maximum_bytes: 65_536 },
    encrypted_secret_provider: {
      provider_id: "local-aes-gcm", maximum_secret_bytes: 2048,
      keys: [{ key_ref: "KEY-0123456789ABCDEF", path: join(root, "key.bin") }],
      secrets: [{
        secret_ref: "SECRET-0123456789ABCDEF", key_ref: "KEY-0123456789ABCDEF",
        path: join(root, "secret.bin")
      }]
    },
    ca_bundles: {
      maximum_bytes: 131_072,
      entries: [{ ca_bundle_ref: "CA-0123456789ABCDEF", path: join(root, "ca.pem") }]
    },
    tls: { connect_timeout_ms: 3000 }
  };
}

function routeManifest(toolName = "calendar.read_event") {
  return {
    schema_version: "dipole.agent.external-mcp-deployment-routes.v1",
    routes: [{
      route_id: "calendar-event-read", route_version: 1, capability_id: "calendar.event.read",
      workflow_step: 4, ordinal: 1, profile_id: "calendar-prod", server_id: "calendar.example",
      tool_name: toolName,
      egress_policy: { allowed_argument_names: ["calendarId", "eventId"], maximum_bytes: 1024 }
    }]
  };
}

function owner() {
  return { expectedOwnerUid: process.getuid!() };
}
