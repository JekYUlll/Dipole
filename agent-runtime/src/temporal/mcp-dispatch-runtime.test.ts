import { createHash } from "node:crypto";
import type { Transport } from "@modelcontextprotocol/client";
import { z } from "zod";
import { describe, expect, it, vi } from "vitest";

import type {
  AgentArtifactCreateInput,
  AgentArtifactRecord,
  AgentMcpToolCommand,
  AgentMcpToolRoundFinish
} from "../capabilities/agent-capability-rpc.js";
import type { McpActivityModernClient } from "../mcp/mcp-input-required-activity.js";
import type { ExternalMcpConfig } from "../mcp/external-mcp-profile.js";
import type { ExternalMcpProductionIoConfig } from "../mcp/external-mcp-production-io.js";
import { ExternalMcpCapabilityRouteRegistry } from "../mcp/mcp-invocation-producer.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import type { TemporalMcpDispatchCheckpointV1 } from "./mcp-dispatch-activity.js";
import { createTemporalMcpDispatchRuntime, type TemporalMcpDispatchRuntimeCore } from "./mcp-dispatch-runtime.js";

const route = {
  routeId: "calendar-event-read", routeVersion: 1, capabilityId: "calendar.event.read", workflowStep: 3, ordinal: 1
};
const context: ExecutionContext = {
  tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId: "TASK-1", runId: "RUN-1",
  mode: "shadow", permissions: ["calendar.read"],
  resourceScopes: [{ resourceType: "calendar", resourceId: "CAL-1", actions: ["read"] }],
  approvedCapabilities: [], requestId: "REQ-1", traceId: "TRACE-1"
};
const argumentsValue = { calendarId: "CAL-1", eventId: "EV-1" };

describe("Temporal MCP dispatch Runtime composition", () => {
  it("composes one route authority and converges Activity completion-loss replay", async () => {
    const harness = runtimeHarness(async () => ({ content: [{ type: "text" as const, text: "meeting" }] }));
    const runtime = createTemporalMcpDispatchRuntime(route, harness.dependencies);
    const input = {
      kind: "begin" as const, ...runtime.routeBinding,
      taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100", arguments: argumentsValue,
      requestId: "REQ-1", traceId: "TRACE-1"
    };

    const first = await runtime.activities.executeMcpDispatch(input);
    const replay = await runtime.activities.executeMcpDispatch(input);

    expect(replay).toEqual(first);
    expect(first).toMatchObject({
      kind: "complete",
      output: {
        invocationId: expect.stringMatching(/^[a-f0-9]{64}$/),
        roundId: expect.stringMatching(/^[a-f0-9]{64}$/),
        artifactId: expect.stringMatching(/^[a-f0-9]{64}$/), artifactVersion: 1
      }
    });
    expect(harness.connectTransport).toHaveBeenCalledTimes(1);
    expect(harness.callToolRound).toHaveBeenCalledTimes(1);
    expect(harness.createArtifact).toHaveBeenCalledTimes(2);
    expect(harness.beginMcpToolCommand.mock.calls[0]?.[0]).toMatchObject({
      profileId: "calendar-prod", serverId: "calendar.example", toolName: "calendar.read_event",
      capabilityId: "calendar.event.read"
    });
    expect(Object.keys(runtime).sort()).toEqual(["activities", "routeBinding"]);
  });

  it("resumes a durable input round through fresh Context and the same Invocation", async () => {
    let round = 0;
    const harness = runtimeHarness(async () => round++ === 0 ? inputRequired() : ({
      content: [{ type: "text" as const, text: "meeting" }]
    }));
    const runtime = createTemporalMcpDispatchRuntime(route, harness.dependencies);
    const wait = await runtime.activities.executeMcpDispatch({
      kind: "begin", ...runtime.routeBinding,
      taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100", arguments: argumentsValue
    });
    if (wait.kind !== "wait_input") throw new Error("expected wait_input");

    const completed = await runtime.activities.executeMcpDispatch({
      kind: "resume", checkpoint: wait.checkpoint as TemporalMcpDispatchCheckpointV1,
      resume: { kind: "input", requestId: wait.requestId, value: { choice: "yes" } }
    });

    expect(completed).toMatchObject({ kind: "complete", output: { artifactId: expect.any(String) } });
    expect(harness.resolveMcpContext).toHaveBeenCalledTimes(2);
    expect(harness.beginMcpToolCommand).toHaveBeenCalledTimes(2);
    expect(harness.connectTransport).toHaveBeenCalledTimes(2);
    expect(harness.createArtifact).toHaveBeenCalledOnce();
  });

  it("honors cancellation before Core, receipt, transport, and Artifact access", async () => {
    const controller = new AbortController();
    controller.abort(new Error("cancelled before Runtime dispatch"));
    const harness = runtimeHarness(async () => ({ content: [] }), () => controller.signal);
    const runtime = createTemporalMcpDispatchRuntime(route, harness.dependencies);

    await expect(runtime.activities.executeMcpDispatch({
      kind: "begin", ...runtime.routeBinding,
      taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100", arguments: argumentsValue
    })).rejects.toThrow(/cancelled before Runtime dispatch/i);
    expect(harness.resolveMcpContext).not.toHaveBeenCalled();
    expect(harness.claimMcpToolRound).not.toHaveBeenCalled();
    expect(harness.connectTransport).not.toHaveBeenCalled();
    expect(harness.createArtifact).not.toHaveBeenCalled();
  });
});

function routes(): ExternalMcpCapabilityRouteRegistry {
  const registry = new ExternalMcpCapabilityRouteRegistry();
  registry.register({
    descriptor: { id: "calendar.event.read", risk: "read", requiredPermission: "calendar.read" },
    inputSchema: z.object({ calendarId: z.string(), eventId: z.string() }).strict(),
    profileId: "calendar-prod", serverId: "calendar.example", toolName: "calendar.read_event",
    egressPolicy: { allowedArgumentNames: ["calendarId", "eventId"], maximumBytes: 1024 },
    resolveResource: input => ({ resourceType: "calendar", resourceId: input.calendarId, action: "read" })
  });
  return registry;
}

function runtimeHarness(
  call: McpActivityModernClient["callToolRound"],
  cancellationSignal: () => AbortSignal = () => new AbortController().signal
) {
  let command: AgentMcpToolCommand | undefined;
  const receipts = new Map<string, Extract<AgentMcpToolRoundFinish, { status: "completed" }>>();
  const resolveMcpContext = vi.fn(async () => context);
  const beginMcpToolCommand = vi.fn(async input => {
    if (command === undefined) {
      command = {
        invocationId: input.invocationId, tenantId: "dipole", principalUserId: "U100", agentId: "UAI",
        taskId: input.taskId, runId: input.runId, profileId: input.profileId!, serverId: input.serverId!,
        toolName: input.toolName, capabilityId: input.capabilityId,
        arguments: JSON.parse(input.argumentsJson!) as Record<string, unknown>, argumentsSha256: input.argumentsSha256,
        startedAtUnixMs: 1_000, status: "running"
      };
    }
    return { invocationId: input.invocationId, status: command.status };
  });
  const resolveMcpToolCommand = vi.fn(async () => {
    if (command === undefined) throw new Error("command missing");
    return command;
  });
  const claimMcpToolRound = vi.fn(async input => {
    const receipt = receipts.get(input.roundId);
    return receipt === undefined ? { outcome: "claimed" as const } : {
      outcome: "replay_completed" as const,
      result: JSON.parse(receipt.resultJSON) as unknown,
      resultJSON: receipt.resultJSON,
      resultSha256: receipt.resultSha256
    };
  });
  const finishMcpToolRound = vi.fn(async (input: AgentMcpToolRoundFinish) => {
    if (input.status === "completed") receipts.set(input.roundId, input);
  });
  const finishMcpToolInvocationFromRound = vi.fn(async input => {
    if (command === undefined || command.invocationId !== input.invocationId) throw new Error("command conflict");
    command = { ...command, status: "completed" };
    return { invocationId: input.invocationId, status: "completed" as const };
  });
  const core: TemporalMcpDispatchRuntimeCore = {
    resolveMcpContext, beginMcpToolCommand, resolveMcpToolCommand,
    claimMcpToolRound, finishMcpToolRound, finishMcpToolInvocationFromRound,
    resolveFreshMcpReadinessEvidence: vi.fn(async (_tenantId, profileBindingSha256, runtimeBindingSha256) => ({
      evidenceId: "e".repeat(64), profileBindingSha256, runtimeBindingSha256, contentSha256: "c".repeat(64),
      collectedAt: "2026-08-28T14:00:00.000Z", expiresAt: "2026-08-28T14:30:00.000Z"
    }))
  };
  const connectTransport = vi.fn(async () => ({ close: vi.fn() }) as unknown as Transport);
  const callToolRound = vi.fn(call);
  const createClient = vi.fn((): McpActivityModernClient => ({
    connect: vi.fn(async () => []), callToolRound, close: vi.fn(async () => undefined)
  }));
  const createArtifact = vi.fn(async (input: AgentArtifactCreateInput) => artifactRecord(input));
  return {
    dependencies: {
      routes: routes(), core,
      externalMcp: {
        config: externalMcpConfig(),
        io: externalMcpIo(),
        registry: {
          describe: () => externalMcpConfig().profiles[0]!,
          connect: connectTransport
        },
        readinessBindingOptions: { expectedOwnerUid: 1000, trustedTransportBuilder: true }
      },
      artifacts: { createArtifact }, createClient, now: () => 1_100,
      ownerTokenSha256: () => "e".repeat(64), cancellationSignal
    },
    resolveMcpContext, beginMcpToolCommand, claimMcpToolRound,
    connectTransport, callToolRound, createArtifact
  };
}

function externalMcpConfig(): Extract<ExternalMcpConfig, { enabled: true }> {
  return {
    enabled: true,
    profiles: [{
      profileId: "calendar-prod", tenantId: "dipole", serverId: "calendar.example",
      endpoint: "https://calendar.example/v1", credentialRef: "CRED-0123456789ABCDEF", credentialVersion: 1,
      allowedHosts: ["calendar.example"], allowedPorts: [443], dnsResolution: "public_only",
      tlsServerName: "calendar.example", caBundleRef: "CA-0123456789ABCDEF", allowedTools: ["calendar.read_event"]
    }]
  };
}

function externalMcpIo(): ExternalMcpProductionIoConfig {
  return {
    credentialCatalogPath: "/run/dipole/catalog.json",
    secretProvider: {
      providerId: "local-aes-gcm",
      keys: { "KEY-0123456789ABCDEF": "/run/dipole/key.bin" },
      secrets: { "SECRET-0123456789ABCDEF": { keyRef: "KEY-0123456789ABCDEF", path: "/run/dipole/secret.bin" } }
    },
    caBundles: { "CA-0123456789ABCDEF": "/run/dipole/ca.pem" }
  };
}

function artifactRecord(input: AgentArtifactCreateInput): AgentArtifactRecord {
  const contentSha256 = sha(input.content);
  const artifactId = sha(Buffer.from([
    "dipole.agent.artifact.v1", input.taskId, input.runId, input.artifactType, input.version.toString(), contentSha256
  ].join("\n"), "utf8"));
  return {
    schemaVersion: "dipole.agent.artifact.v1", artifactId, taskId: input.taskId, runId: input.runId,
    artifactType: input.artifactType, version: input.version, title: input.title, mediaType: input.mediaType,
    contentSha256, sizeBytes: input.content.byteLength, metadata: input.metadata
  };
}

function inputRequired() {
  return {
    resultType: "input_required" as const,
    inputRequests: {
      "event-choice": {
        method: "elicitation/create" as const,
        params: {
          mode: "form" as const, message: "Choose",
          requestedSchema: {
            type: "object" as const,
            properties: { choice: { type: "string" as const, title: "Choice" } },
            required: ["choice"]
          }
        }
      }
    },
    requestState: "opaque-state"
  };
}

function sha(value: Uint8Array): string {
  return createHash("sha256").update(value).digest("hex");
}
