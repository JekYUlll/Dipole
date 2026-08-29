import { createHash } from "node:crypto";
import { describe, expect, it, vi } from "vitest";

import type {
  AgentArtifactCreateInput,
  AgentArtifactRecord,
  AgentMcpToolCommand
} from "../capabilities/agent-capability-rpc.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import { canonicalMcpJSON } from "./canonical-json.js";
import {
  ExternalMcpArtifactProjector,
  type ExternalMcpArtifactCommandResolver,
  type ExternalMcpArtifactWriter
} from "./external-mcp-artifact-projector.js";

const invocationId = "a".repeat(64);
const roundId = "b".repeat(64);
const context: ExecutionContext = {
  tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId: "TASK-1", runId: "RUN-1",
  mode: "shadow", permissions: ["calendar.read"],
  resourceScopes: [{ resourceType: "calendar", resourceId: "CAL-1", actions: ["read"] }],
  approvedCapabilities: [], requestId: "REQ-1", traceId: "TRACE-1"
};
const command: AgentMcpToolCommand = {
  invocationId, tenantId: "dipole", principalUserId: "U100", agentId: "UAI",
  taskId: "TASK-1", runId: "RUN-1", profileId: "calendar-prod", serverId: "calendar.example",
  toolName: "calendar.read_event", capabilityId: "calendar.event.read", arguments: { eventId: "EV-1" },
  argumentsSha256: "d".repeat(64), startedAtUnixMs: 1_000, status: "completed"
};
const result = { content: [{ type: "text" as const, text: "meeting at 10" }], structuredContent: { eventId: "EV-1" } };

describe("External MCP Artifact projector", () => {
  it("reloads terminal command authority and writes an untrusted content-addressed Artifact", async () => {
    const dependencies = projectorDependencies();
    const projector = new ExternalMcpArtifactProjector(dependencies.commands, dependencies.artifacts);

    await expect(projector.project({ context, invocationId, roundId, result })).resolves.toMatchObject({
      artifactId: expect.stringMatching(/^[a-f0-9]{64}$/), artifactVersion: 1
    });

    expect(dependencies.commands.resolveMcpToolCommand).toHaveBeenCalledWith("TASK-1", "RUN-1", invocationId);
    expect(dependencies.artifacts.createArtifact).toHaveBeenCalledWith({
      tenantId: "dipole", taskId: "TASK-1", runId: "RUN-1",
      artifactType: `mcp_result.${invocationId.slice(0, 53)}`, version: 1,
      title: "External MCP result: calendar.example/calendar.read_event", mediaType: "application/json",
      content: Buffer.from(canonicalMcpJSON(result), "utf8"),
      metadata: {
        schemaVersion: "dipole.agent.external-mcp-result.v1", trust: "untrusted", sourceType: "external_mcp_tool",
        invocationId, roundId, profileId: "calendar-prod", serverId: "calendar.example",
        toolName: "calendar.read_event", capabilityId: "calendar.event.read",
        argumentsSha256: "d".repeat(64), resultSha256: sha(Buffer.from(canonicalMcpJSON(result), "utf8"))
      },
      requestId: "REQ-1", traceId: "TRACE-1"
    });
  });

  it("converges exact Activity completion replay on the same Artifact", async () => {
    const dependencies = projectorDependencies();
    const projector = new ExternalMcpArtifactProjector(dependencies.commands, dependencies.artifacts);

    const first = await projector.project({ context, invocationId, roundId, result });
    const replay = await projector.project({ context, invocationId, roundId, result });

    expect(replay).toEqual(first);
    expect(dependencies.commands.resolveMcpToolCommand).toHaveBeenCalledTimes(2);
    expect(dependencies.artifacts.createArtifact).toHaveBeenCalledTimes(2);
    expect(dependencies.artifacts.createArtifact.mock.calls[1]?.[0]).toEqual(
      dependencies.artifacts.createArtifact.mock.calls[0]?.[0]
    );
  });

  it("rejects active Context, non-terminal command, and authority drift before Artifact creation", async () => {
    const active = projectorDependencies();
    const activeProjector = new ExternalMcpArtifactProjector(active.commands, active.artifacts);
    await expect(activeProjector.project({ context: { ...context, mode: "active" }, invocationId, roundId, result }))
      .rejects.toThrow(/input is invalid/i);
    expect(active.commands.resolveMcpToolCommand).not.toHaveBeenCalled();

    for (const changed of [
      { ...command, status: "running" as const },
      { ...command, profileId: "" },
      { ...command, principalUserId: "U200" }
    ]) {
      const dependencies = projectorDependencies(changed);
      await expect(new ExternalMcpArtifactProjector(dependencies.commands, dependencies.artifacts).project({
        context, invocationId, roundId, result
      })).rejects.toThrow(/command binding/i);
      expect(dependencies.artifacts.createArtifact).not.toHaveBeenCalled();
    }
  });

  it("rejects failed, malformed, transformed, and oversized MCP results before Artifact creation", async () => {
    const invalid = [
      { content: [], isError: true },
      { content: "invalid" },
      { content: [], invalid: undefined },
      { content: [{ type: "text", text: "x".repeat(129 * 1024) }] }
    ];
    for (const value of invalid) {
      const dependencies = projectorDependencies();
      await expect(new ExternalMcpArtifactProjector(dependencies.commands, dependencies.artifacts).project({
        context, invocationId, roundId, result: value
      })).rejects.toThrow(/result|successful/i);
      expect(dependencies.artifacts.createArtifact).not.toHaveBeenCalled();
    }
  });

  it("stops on cancellation before resolution and after fresh command resolution", async () => {
    const before = new AbortController();
    before.abort(new Error("cancelled before projection"));
    const first = projectorDependencies();
    await expect(new ExternalMcpArtifactProjector(first.commands, first.artifacts).project({
      context, invocationId, roundId, result
    }, before.signal)).rejects.toThrow(/cancelled before projection/i);
    expect(first.commands.resolveMcpToolCommand).not.toHaveBeenCalled();

    const during = new AbortController();
    const second = projectorDependencies();
    second.commands.resolveMcpToolCommand.mockImplementationOnce(async () => {
      during.abort(new Error("cancelled during resolution"));
      return command;
    });
    await expect(new ExternalMcpArtifactProjector(second.commands, second.artifacts).project({
      context, invocationId, roundId, result
    }, during.signal)).rejects.toThrow(/cancelled during resolution/i);
    expect(second.artifacts.createArtifact).not.toHaveBeenCalled();
  });

  it("converges after cancellation observes an already committed Artifact", async () => {
    const controller = new AbortController();
    const dependencies = projectorDependencies();
    dependencies.artifacts.createArtifact.mockImplementationOnce(async input => {
      controller.abort(new Error("cancelled after Artifact commit"));
      return artifactRecord(input);
    });
    const projector = new ExternalMcpArtifactProjector(dependencies.commands, dependencies.artifacts);
    const input = { context, invocationId, roundId, result };

    await expect(projector.project(input, controller.signal)).rejects.toThrow(/cancelled after Artifact commit/i);
    const replay = await projector.project(input, new AbortController().signal);

    expect(replay).toMatchObject({ artifactId: expect.stringMatching(/^[a-f0-9]{64}$/), artifactVersion: 1 });
    expect(dependencies.artifacts.createArtifact).toHaveBeenCalledTimes(2);
    expect(dependencies.artifacts.createArtifact.mock.calls[1]?.[0]).toEqual(
      dependencies.artifacts.createArtifact.mock.calls[0]?.[0]
    );
  });

  it("rejects conflicting Artifact writer evidence", async () => {
    const dependencies = projectorDependencies();
    dependencies.artifacts.createArtifact.mockImplementationOnce(async input => ({
      ...artifactRecord(input), artifactId: "f".repeat(64)
    }));
    await expect(new ExternalMcpArtifactProjector(dependencies.commands, dependencies.artifacts).project({
      context, invocationId, roundId, result
    })).rejects.toThrow(/conflicting evidence/i);
  });
});

function projectorDependencies(commandValue: AgentMcpToolCommand = command) {
  const resolveMcpToolCommand = vi.fn<ExternalMcpArtifactCommandResolver["resolveMcpToolCommand"]>(async () => commandValue);
  const createArtifact = vi.fn<ExternalMcpArtifactWriter["createArtifact"]>(async input => artifactRecord(input));
  return { commands: { resolveMcpToolCommand }, artifacts: { createArtifact } };
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

function sha(value: Uint8Array): string {
  return createHash("sha256").update(value).digest("hex");
}
