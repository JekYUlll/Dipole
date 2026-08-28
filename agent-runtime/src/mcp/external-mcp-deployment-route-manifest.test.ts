import { chmod, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it, vi } from "vitest";
import { z } from "zod";

import type { ExternalMcpConfig } from "./external-mcp-profile.js";
import {
  ExternalMcpCapabilityDefinitionRegistry,
  externalMcpDeploymentRouteManifestSchemaVersion,
  loadExternalMcpDeploymentRouteManifest
} from "./external-mcp-deployment-route-manifest.js";

const directories: string[] = [];

afterEach(async () => {
  await Promise.all(directories.splice(0).map(path => rm(path, { recursive: true, force: true })));
});

describe("external MCP deployment route manifest", () => {
  it("keeps the language-neutral schema aligned and credential-free", async () => {
    const schemaPath = new URL("../../../contracts/agent-external-mcp/v1/deployment-route-manifest.schema.json", import.meta.url);
    const schema = JSON.parse(await readFile(schemaPath, "utf8")) as {
      $id: string;
      "x-dipole-version": string;
      properties: { schema_version: { const: string } };
    };
    expect(schema.$id).toMatch(/agent-external-mcp\/v1\/deployment-route-manifest\.schema\.json$/);
    expect(schema["x-dipole-version"]).toBe(externalMcpDeploymentRouteManifestSchemaVersion);
    expect(schema.properties.schema_version.const).toBe(externalMcpDeploymentRouteManifestSchemaVersion);
    expect(JSON.stringify(schema)).not.toMatch(/credential_ref|secret|token|principal|task_id|run_id/i);
  });

  it("does not read residual route configuration while external Profiles are disabled", async () => {
    const env = {} as NodeJS.ProcessEnv;
    Object.defineProperty(env, "DIPOLE_AGENT_EXTERNAL_MCP_ROUTE_MANIFEST", {
      get: () => { throw new Error("disabled route loader touched residual configuration"); }
    });
    await expect(loadExternalMcpDeploymentRouteManifest(
      { enabled: false, profiles: [] }, definitions(), env
    )).resolves.toBeUndefined();
  });

  it("joins one deployment route to an exact code definition and enabled Profile", async () => {
    const path = await writeManifest(manifest());
    const loaded = await loadExternalMcpDeploymentRouteManifest(enabledConfig(), definitions(), env(path), {
      expectedOwnerUid: process.getuid!()
    });

    expect(loaded?.routes).toEqual([{
      routeId: "calendar-event-read", routeVersion: 3, capabilityId: "calendar.event.read",
      workflowStep: 4, ordinal: 1, deploymentBindingSha256: expect.stringMatching(/^[a-f0-9]{64}$/)
    }]);
    expect(loaded?.registry.workerEgressPolicies("calendar.event.read")).toEqual({
      "calendar-prod": {
        "calendar.read_event": { allowedArgumentNames: ["calendarId", "eventId"], maximumBytes: 1024 }
      }
    });
    expect(loaded?.registry.prepare("calendar.event.read", {
      calendarId: "CAL-1", eventId: "EV-1"
    }, executionContext())).toMatchObject({
      profileId: "calendar-prod", serverId: "calendar.example", toolName: "calendar.read_event"
    });
  });

  it("allows deployment policy to narrow a code ceiling and rejects any expansion", async () => {
    const narrowed = manifest();
    narrowed.routes[0]!.egress_policy = { allowed_argument_names: ["calendarId"], maximum_bytes: 512 };
    const narrowPath = await writeManifest(narrowed);
    const loaded = await loadExternalMcpDeploymentRouteManifest(enabledConfig(), definitions(), env(narrowPath), owner());
    expect(loaded?.registry.workerEgressPolicies("calendar.event.read")).toMatchObject({
      "calendar-prod": { "calendar.read_event": { allowedArgumentNames: ["calendarId"], maximumBytes: 512 } }
    });
    const baselinePath = await writeManifest(manifest());
    const baseline = await loadExternalMcpDeploymentRouteManifest(enabledConfig(), definitions(), env(baselinePath), owner());
    expect(loaded?.routes[0]?.deploymentBindingSha256).not.toBe(baseline?.routes[0]?.deploymentBindingSha256);
    const reordered = manifest();
    reordered.routes[0]!.egress_policy.allowed_argument_names.reverse();
    const reorderedPath = await writeManifest(reordered);
    const equivalent = await loadExternalMcpDeploymentRouteManifest(enabledConfig(), definitions(), env(reorderedPath), owner());
    expect(equivalent?.routes[0]?.deploymentBindingSha256).toBe(baseline?.routes[0]?.deploymentBindingSha256);

    const cases = [
      { ...manifest(), routes: [{ ...manifest().routes[0]!, egress_policy: {
        allowed_argument_names: ["calendarId", "eventId", "adminOverride"], maximum_bytes: 1024
      } }] },
      { ...manifest(), routes: [{ ...manifest().routes[0]!, egress_policy: {
        allowed_argument_names: ["calendarId"], maximum_bytes: 4097
      } }] }
    ];
    for (const candidate of cases) {
      const path = await writeManifest(candidate);
      await expect(loadExternalMcpDeploymentRouteManifest(enabledConfig(), definitions(), env(path), owner()))
        .rejects.toThrow("External MCP deployment route manifest is unavailable");
    }
  });

  it("rejects Profile, Server, Tool, definition and duplicate coordinate drift", async () => {
    const base = manifest().routes[0]!;
    const cases = [
      [{ ...base, profile_id: "missing" }],
      [{ ...base, server_id: "different.example" }],
      [{ ...base, tool_name: "calendar.delete_event" }],
      [{ ...base, capability_id: "calendar.unknown" }],
      [base, { ...base, route_id: "another-route", capability_id: "calendar.event.read" }],
      [base, { ...base, route_id: "another-route", capability_id: "calendar.event.other" }],
      [base, { ...base, route_id: "calendar-event-read", capability_id: "calendar.event.other", workflow_step: 5 }]
    ];
    const registry = definitions();
    registry.register({
      descriptor: { id: "calendar.event.other", risk: "read", requiredPermission: "calendar.read" },
      inputSchema: z.object({ calendarId: z.string() }).strict(),
      egressCeiling: { allowedArgumentNames: ["calendarId"], maximumBytes: 4096 },
      resolveResource: input => ({ resourceType: "calendar", resourceId: input.calendarId, action: "read" })
    });
    for (const routes of cases) {
      const path = await writeManifest({ ...manifest(), routes });
      await expect(loadExternalMcpDeploymentRouteManifest(enabledConfig(), registry, env(path), owner()))
        .rejects.toThrow("External MCP deployment route manifest is unavailable");
    }
  });

  it("requires canonical owner-only file evidence and propagates cancellation", async () => {
    const path = await writeManifest(manifest());
    await chmod(path, 0o644);
    await expect(loadExternalMcpDeploymentRouteManifest(enabledConfig(), definitions(), env(path), owner()))
      .rejects.toThrow("External MCP deployment route manifest is unavailable");

    const controller = new AbortController();
    controller.abort(new Error("cancelled before route manifest"));
    const untouched = {} as NodeJS.ProcessEnv;
    Object.defineProperty(untouched, "DIPOLE_AGENT_EXTERNAL_MCP_ROUTE_MANIFEST", {
      get: vi.fn(() => path)
    });
    await expect(loadExternalMcpDeploymentRouteManifest(enabledConfig(), definitions(), untouched, {
      ...owner(), signal: controller.signal
    })).rejects.toThrow(/cancelled before route manifest/i);
  });

  it("rejects invalid or duplicated code definitions before reading deployment state", () => {
    const registry = definitions();
    expect(() => registry.register({
      descriptor: { id: "calendar.event.read", risk: "read", requiredPermission: "calendar.read" },
      inputSchema: z.object({}).strict(),
      egressCeiling: { allowedArgumentNames: [], maximumBytes: 128 },
      resolveResource: () => ({ resourceType: "calendar", resourceId: "*", action: "read" })
    })).toThrow(/definition ID/i);
    expect(() => new ExternalMcpCapabilityDefinitionRegistry().register({
      descriptor: { id: "calendar.event.read", risk: "read", requiredPermission: "" },
      inputSchema: z.object({}).strict(),
      egressCeiling: { allowedArgumentNames: [], maximumBytes: 128 },
      resolveResource: () => ({ resourceType: "calendar", resourceId: "*", action: "read" })
    })).toThrow(/descriptor/i);
  });
});

function definitions() {
  const registry = new ExternalMcpCapabilityDefinitionRegistry();
  registry.register({
    descriptor: { id: "calendar.event.read", risk: "read", requiredPermission: "calendar.read" },
    inputSchema: z.object({ calendarId: z.string(), eventId: z.string().optional() }).strict(),
    egressCeiling: { allowedArgumentNames: ["calendarId", "eventId"], maximumBytes: 4096 },
    resolveResource: input => ({ resourceType: "calendar", resourceId: input.calendarId, action: "read" })
  });
  return registry;
}

function enabledConfig(): Extract<ExternalMcpConfig, { enabled: true }> {
  return {
    enabled: true,
    profiles: [{
      profileId: "calendar-prod", tenantId: "dipole", serverId: "calendar.example",
      endpoint: "https://calendar.example/v1", credentialRef: "CRED-0123456789ABCDEF", credentialVersion: 1,
      allowedHosts: ["calendar.example"], allowedPorts: [443], dnsResolution: "public_only",
      tlsServerName: "calendar.example", caBundleRef: "CA-0123456789ABCDEF",
      allowedTools: ["calendar.read_event"]
    }]
  };
}

function manifest() {
  return {
    schema_version: externalMcpDeploymentRouteManifestSchemaVersion,
    routes: [{
      route_id: "calendar-event-read", route_version: 3, capability_id: "calendar.event.read",
      workflow_step: 4, ordinal: 1, profile_id: "calendar-prod", server_id: "calendar.example",
      tool_name: "calendar.read_event",
      egress_policy: { allowed_argument_names: ["calendarId", "eventId"], maximum_bytes: 1024 }
    }]
  };
}

async function writeManifest(value: unknown): Promise<string> {
  const directory = await mkdtemp(join(tmpdir(), "dipole-mcp-route-manifest-"));
  directories.push(directory);
  const path = join(directory, "routes.json");
  await writeFile(path, JSON.stringify(value), { mode: 0o600 });
  return path;
}

function env(path: string): NodeJS.ProcessEnv {
  return { DIPOLE_AGENT_EXTERNAL_MCP_ROUTE_MANIFEST: path };
}

function owner() {
  return { expectedOwnerUid: process.getuid!() };
}

function executionContext() {
  return {
    tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId: "TASK-1", runId: "RUN-1",
    mode: "shadow" as const, permissions: ["calendar.read"],
    resourceScopes: [{ resourceType: "calendar", resourceId: "CAL-1", actions: ["read"] }],
    approvedCapabilities: []
  };
}
