import { describe, expect, it, vi } from "vitest";
import { createHash } from "node:crypto";

import type { IAgentCapabilityServiceClient } from "../generated/dipole/agent/v1/agent.grpc-client.js";
import { AgentCapabilityRPCClient } from "./agent-capability-rpc.js";

describe("AgentCapabilityRPCClient", () => {
  it("admits from trusted event identity and lists by Task/Run only", async () => {
    const admitRun = vi.fn((_input, metadata, _options, callback) => {
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      expect(metadata.get("x-dipole-service-token")).toEqual(["secret"]);
      callback(null, { taskId: "TASK-1", runId: "RUN-1", runStatus: "running" });
      return {};
    });
    const completeRun = vi.fn((input, metadata, _options, callback) => {
      expect(input).toMatchObject({ taskId: "TASK-1", runId: "RUN-1" });
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, { runStatus: "completed" });
      return {};
    });
    const matchEventSubscriptions = vi.fn((input, metadata, _options, callback) => {
      expect(input).toMatchObject({
        tenantId: "dipole", agentId: "UAI", eventType: "message.direct.created",
        resourceType: "conversation", resourceId: "direct:U100:UAI"
      });
      expect(input.context?.principalUserId).toBe("");
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, { subscriptions: [{
        subscriptionId: "SUB-1", definitionId: "DEF-1", definitionVersion: 2n,
        tenantId: "dipole", agentId: "UAI", eventType: "message.direct.created",
        resourceType: "conversation", resourceId: "direct:U100:UAI",
        filterKind: "message_contains_any", filterJson: Buffer.from(JSON.stringify({ terms: ["hello"] }))
      }] });
      return {};
    });
    const listContextMemories = vi.fn((input, metadata, _options, callback) => {
      expect(input).toMatchObject({ taskId: "TASK-1", runId: "RUN-1", resourceType: "conversation", resourceId: "direct:U100:UAI", limit: 20 });
      expect(input.context?.principalUserId).toBe("");
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, { memories: [{
        memoryId: "MEM-1", memoryType: "semantic", content: "Owner is Alice", compactContent: "Owner: Alice", priority: 90,
        provenance: { sourceType: "message", sourceId: "M1", uri: "dipole://message/M1", sequence: "42" }
      }] });
      return {};
    });
    const finishRun = vi.fn((input, metadata, _options, callback) => {
      expect(input).toMatchObject({
        taskId: "TASK-1",
        runId: "RUN-1",
        runStatus: "failed",
        lastError: "Activity retries exhausted"
      });
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, { runStatus: "failed" });
      return {};
    });
    const resolveMcpToolCommand = vi.fn((input, metadata, _options, callback) => {
      expect(input).toMatchObject({ taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-EXT-1" });
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      const argumentsJson = Buffer.from(`{"calendarId":"CAL-1"}`);
      callback(null, {
        invocationId: "INV-EXT-1", tenantId: "dipole", principalUserId: "U100", agentId: "UAI", taskId: "TASK-1", runId: "RUN-1",
        profileId: "calendar-prod", serverId: "calendar.example", toolName: "calendar.create", capabilityId: "conversation.list",
        argumentsJson, argumentsSha256: createHash("sha256").update(argumentsJson).digest("hex"), startedAtUnixMs: 1_000n
      });
      return {};
    });
    const requestApproval = vi.fn((input, _metadata, _options, callback) => {
      expect(input).toMatchObject({ taskId: "TASK-1", runId: "RUN-1", approvalId: "APR-1", capabilityId: "message.bulk.send" });
      callback(null, { approvalId: "APR-1", status: "pending", approvedByUserId: "" });
      return {};
    });
    const resolveApproval = vi.fn((input, _metadata, _options, callback) => {
      expect(input).toMatchObject({ taskId: "TASK-1", runId: "RUN-1", approvalId: "APR-1", actorUserId: "U100", decision: "approved" });
      callback(null, { approvalId: "APR-1", status: "approved", approvedByUserId: "U100" });
      return {};
    });
    const consumeApproval = vi.fn((input, metadata, _options, callback) => {
      expect(input).toMatchObject({
        taskId: "TASK-1", runId: "RUN-1", approvalId: "APR-1", capabilityId: "message.bulk.send",
        scopeSha256: "a".repeat(64), argumentsSha256: "b".repeat(64), nonceSha256: "c".repeat(64), mode: "active"
      });
      expect(input.context?.principalUserId).toBe("");
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, { approvalId: input.approvalId, status: "consumed" });
      return {};
    });
    const resolveApprovalGrant = vi.fn((input, metadata, _options, callback) => {
      expect(input).toMatchObject({
        taskId: "TASK-1", runId: "RUN-1", capabilityId: "message.bulk.send",
        resourceScope: { resourceType: "conversation", resourceId: "G1", actions: ["write"] },
        argumentsSha256: "b".repeat(64)
      });
      expect(input.context?.principalUserId).toBe("");
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, {
        approvalId: "APR-1", capabilityId: input.capabilityId, resourceScope: input.resourceScope,
        scopeSha256: "bd1b1e13995be9b2d7e32b93f9391268f946755a3be8cf977e8763ab4bb3aced",
        argumentsSha256: input.argumentsSha256, nonceSha256: "c".repeat(64), expiresAtUnixMs: BigInt(Date.now() + 60_000)
      });
      return {};
    });
    const listConversations = vi.fn((input, _metadata, _options, callback) => {
      expect(input.context?.principalUserId).toBe("");
      expect(input).toMatchObject({ taskId: "TASK-1", runId: "RUN-1", limit: 20 });
      callback(null, { conversations: [{
        conversationKey: "group:G1", targetId: "G1", targetType: 1, lastMessageId: "M1",
        lastMessageSeq: 42n, lastMessagePreview: "hello", lastMessageAtUnixMs: 1000n, readSeq: 40n, unreadCount: 2
      }] });
      return {};
    });
    const authorizeTaskControl = vi.fn((input, metadata, _options, callback) => {
      expect(input.context?.principalUserId).toBe("");
      expect(input).toMatchObject({ taskId: "TASK-1", principalUserId: "U100" });
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, {
        taskId: "TASK-1", taskStatus: "running", workflowId: "dipole-agent-task/TASK-1",
        workflowRunId: "temporal-run-1", workflowStatus: "waiting_approval", workflowRevision: 2n
      });
      return {};
    });
    const resolveMcpContext = vi.fn((input, metadata, _options, callback) => {
      expect(input.context?.principalUserId).toBe("");
      expect(input).toMatchObject({ taskId: "TASK-1", runId: "RUN-1", principalUserId: "U100" });
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, {
        tenantId: "dipole", principalUserId: "U100", agentId: "UAI", delegatedByUserId: "U100",
        runtimeId: "dipole-agent", mode: "active",
        permissions: ["conversation.list"], resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["list"] }],
        approvedCapabilities: []
      });
      return {};
    });
    const projectTaskWorkflowState = vi.fn((input, metadata, _options, callback) => {
      expect(input.context?.principalUserId).toBe("");
      expect(input).toMatchObject({
        taskId: "TASK-1", runId: "RUN-1", workflowId: "dipole-agent-task/TASK-1",
        workflowRunId: "temporal-run-1", workflowStatus: "waiting_approval", workflowRevision: 2n
      });
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, {
        taskId: input.taskId, workflowId: input.workflowId, workflowRunId: input.workflowRunId,
        workflowStatus: input.workflowStatus, workflowRevision: input.workflowRevision
      });
      return {};
    });
    const listTaskWorkflowProjectionSnapshots = vi.fn((input, metadata, _options, callback) => {
      expect(input).toMatchObject({ afterTaskId: "TASK-0", pageSize: 100 });
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, { tasks: [{
        taskId: "TASK-1", hasWorkflow: true, workflowId: "dipole-agent-task/TASK-1",
        workflowRunId: "temporal-run-1", workflowStatus: "running", workflowRevision: 1n
      }, { taskId: "TASK-2", hasWorkflow: false }], nextCursor: "TASK-2" });
      return {};
    });
    const createArtifact = vi.fn((input, metadata, _options, callback) => {
      expect(input.context?.principalUserId).toBe("");
      expect(input).toMatchObject({ taskId: "TASK-1", runId: "RUN-1", artifactType: "report", version: 1 });
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      const contentSha256 = createHash("sha256").update(input.content).digest("hex");
      const artifactId = createHash("sha256").update(["dipole.agent.artifact.v1", input.taskId, input.runId, input.artifactType, input.version.toString(), contentSha256].join("\n")).digest("hex");
      callback(null, { artifact: {
        schemaVersion: "dipole.agent.artifact.v1", artifactId, taskId: input.taskId,
        runId: input.runId, artifactType: input.artifactType, version: input.version,
        title: input.title, mediaType: input.mediaType, contentSha256,
        sizeBytes: BigInt(input.content.byteLength), metadataJson: input.metadataJson, createdAtUnixMs: 1n
      } });
      return {};
    });
    const beginMcpToolInvocation = vi.fn((input, metadata, _options, callback) => {
      expect(input.context?.principalUserId).toBe("");
      expect(input).toMatchObject({ taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-1" });
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, { invocationId: input.invocationId, status: "running" });
      return {};
    });
    const claimMcpToolRound = vi.fn((input, metadata, _options, callback) => {
      expect(input.context?.principalUserId).toBe("");
      expect(input).toMatchObject({ taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-EXT-1", roundNumber: 0 });
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      const resultJSON = `{"content":[]}`;
      callback(null, {
        roundId: input.roundId, outcome: "replay_completed", resultJson: Buffer.from(resultJSON),
        resultSha256: createHash("sha256").update(resultJSON).digest("hex"), errorCode: ""
      });
      return {};
    });
    const finishMcpToolRound = vi.fn((input, metadata, _options, callback) => {
      expect(input.context?.principalUserId).toBe("");
      expect(input.status).toBe("completed");
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, { roundId: input.roundId, status: input.status });
      return {};
    });
    const finishMcpToolInvocation = vi.fn((input, metadata, _options, callback) => {
      expect(input.context?.principalUserId).toBe("");
      expect(input).toMatchObject({ taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-1", status: "completed", resultBytes: 128n, latencyMs: 12n });
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, { invocationId: input.invocationId, status: input.status });
      return {};
    });
    const executeMcpMessageCommand = vi.fn((input, metadata, _options, callback) => {
      const commandId = "tool:" + "c".repeat(64);
      const clientMessageId = createHash("sha256").update(`dipole.agent.command.v1\n${input.commandKind}\n${commandId}`).digest("hex");
      expect(input).toMatchObject({ taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-1", commandKind: "system_message", content: "notice" });
      expect(input.context?.principalUserId).toBe("");
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, { actionReference: { resourceType: "message", resourceId: "MSG-1", commandKind: input.commandKind, commandId }, clientMessageId });
      return {};
    });
    const client = new AgentCapabilityRPCClient({ admitRun, matchEventSubscriptions, listContextMemories, completeRun, finishRun, requestApproval, resolveApproval, consumeApproval, resolveApprovalGrant, listConversations, authorizeTaskControl, resolveMcpContext, beginMcpToolInvocation, resolveMcpToolCommand, claimMcpToolRound, finishMcpToolRound, finishMcpToolInvocation, executeMcpMessageCommand, projectTaskWorkflowState, listTaskWorkflowProjectionSnapshots, createArtifact } as unknown as IAgentCapabilityServiceClient, "secret");
    const identity = { tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", requestId: "R1", traceId: "T1" };
    const event = {
      eventId: "E1", eventType: "message.direct.created", aggregateId: "M1",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: { conversation_key: "direct:U100:UAI" },
      subscriptionId: "SUB-1"
    };

    await expect(client.admit(event, identity)).resolves.toEqual({ taskId: "TASK-1", runId: "RUN-1", runStatus: "running" });
    expect(admitRun).toHaveBeenCalledWith(
      expect.objectContaining({ subscriptionId: "SUB-1" }), expect.anything(), expect.anything(), expect.any(Function)
    );
    await expect(client.matchEventSubscriptions(event, identity)).resolves.toEqual([{
      subscriptionId: "SUB-1", definitionId: "DEF-1", definitionVersion: 2,
      tenantId: "dipole", agentId: "UAI", eventType: "message.direct.created",
      resourceType: "conversation", resourceId: "direct:U100:UAI",
      filterKind: "message_contains_any", filter: { terms: ["hello"] }
    }]);
    await expect(client.listContextMemories({ taskId: "TASK-1", runId: "RUN-1", requestId: "R1", traceId: "T1" }, "conversation", "direct:U100:UAI", 20)).resolves.toEqual([{
      memoryId: "MEM-1", memoryType: "semantic", content: "Owner is Alice", compactContent: "Owner: Alice", priority: 90,
      provenance: { sourceType: "message", sourceId: "M1", uri: "dipole://message/M1", sequence: "42" }
    }]);
    await expect(client.listConversations({
      ...identity, taskId: "TASK-1", runId: "RUN-1", mode: "shadow",
      permissions: ["conversation.list"], resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["list"] }],
      approvedCapabilities: [], eventId: "E1"
    }, 20)).resolves.toEqual([expect.objectContaining({ conversationKey: "group:G1", lastMessageSeq: "42", readSeq: "40" })]);
    await expect(client.complete("TASK-1", "RUN-1", identity)).resolves.toBeUndefined();
    await expect(client.finish(
      "TASK-1", "RUN-1", "failed", "Activity retries exhausted", identity
    )).resolves.toBeUndefined();
    const approval = {
      approvalId: "APR-1", capabilityId: "message.bulk.send",
      resourceScope: { resourceType: "conversation", resourceId: "G1", actions: ["write"] },
      scopeSha256: "a".repeat(64), argumentsSha256: "b".repeat(64), nonceSha256: "c".repeat(64),
      expiresAtUnixMs: Date.UTC(2026, 7, 28)
    };
    await expect(client.requestApproval("TASK-1", "RUN-1", approval, identity)).resolves.toBeUndefined();
    await expect(client.resolveApproval("TASK-1", "RUN-1", "APR-1", "approved", "U100", identity)).resolves.toBeUndefined();
    await expect(client.consumeApproval("TASK-1", "RUN-1", {
      approvalId: "APR-1", capabilityId: "message.bulk.send", scopeSha256: "a".repeat(64),
      argumentsSha256: "b".repeat(64), nonceSha256: "c".repeat(64)
    }, identity)).resolves.toBeUndefined();
    await expect(client.resolveApprovalGrant(
      "TASK-1", "RUN-1", "message.bulk.send",
      { resourceType: "conversation", resourceId: "G1", actions: ["write"] }, "b".repeat(64), identity
    )).resolves.toMatchObject({ approvalId: "APR-1", nonceSha256: "c".repeat(64) });
    await expect(client.authorizeTaskControl("TASK-1", "U100", identity)).resolves.toEqual({
      taskId: "TASK-1", taskStatus: "running",
      workflow: {
        taskId: "TASK-1", workflowId: "dipole-agent-task/TASK-1", workflowRunId: "temporal-run-1",
        workflowStatus: "waiting_approval", workflowRevision: 2
      }
    });
    await expect(client.resolveMcpContext("TASK-1", "RUN-1", "U100", identity)).resolves.toMatchObject({
      tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId: "TASK-1", runId: "RUN-1", mode: "active",
      permissions: ["conversation.list"], resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["list"] }]
    });
    resolveMcpContext.mockImplementationOnce((_input, _metadata, _options, callback) => {
      callback(null, {
        tenantId: "dipole", principalUserId: "U100", agentId: "UAI", delegatedByUserId: "U100",
        runtimeId: "forged-runtime", mode: "active", permissions: ["conversation.list"],
        resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["list"] }], approvedCapabilities: []
      });
      return {};
    });
    await expect(client.resolveMcpContext("TASK-1", "RUN-1", "U100", identity)).rejects.toThrow(/Runtime binding/i);
    await expect(client.begin({
      invocationId: "INV-1", taskId: "TASK-1", runId: "RUN-1", toolName: "dipole_conversation_list",
      capabilityId: "conversation.list", argumentsSha256: "a".repeat(64), requestId: "R1", traceId: "T1"
    })).resolves.toBeUndefined();
    await expect(client.resolveMcpToolCommand("TASK-1", "RUN-1", "INV-EXT-1")).resolves.toMatchObject({
      profileId: "calendar-prod", serverId: "calendar.example", arguments: { calendarId: "CAL-1" }, startedAtUnixMs: 1_000
    });
    const roundId = "d".repeat(64);
    const ownerTokenSha256 = "e".repeat(64);
    await expect(client.claimMcpToolRound({
      taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-EXT-1", roundId, roundNumber: 0,
      requestSha256: "f".repeat(64), ownerTokenSha256
    })).resolves.toMatchObject({ outcome: "replay_completed", result: { content: [] } });
    const roundResultJSON = `{"content":[]}`;
    await expect(client.finishMcpToolRound({
      roundId, ownerTokenSha256, status: "completed", resultJSON: roundResultJSON,
      resultSha256: createHash("sha256").update(roundResultJSON).digest("hex")
    })).resolves.toBeUndefined();
    await expect(client.finishToolInvocation({
      invocationId: "INV-1", taskId: "TASK-1", runId: "RUN-1", status: "completed",
      resultSha256: "b".repeat(64), resultBytes: 128, latencyMs: 12
    })).resolves.toBeUndefined();
    await expect(client.begin({
      invocationId: "INV-1", taskId: "TASK-1", runId: "RUN-1", toolName: "dipole_message_send",
      capabilityId: "message.system.send", argumentsSha256: "a".repeat(64), approvalId: "APR-1"
    })).resolves.toBeUndefined();
    expect(beginMcpToolInvocation.mock.calls.at(-1)?.[0]).toMatchObject({ approvalId: "APR-1" });
    await expect(client.finishToolInvocation({
      invocationId: "INV-1", taskId: "TASK-1", runId: "RUN-1", status: "completed",
      resultSha256: "b".repeat(64), resultBytes: 128, latencyMs: 12,
      actionReference: { resourceType: "message", resourceId: "MSG-1", commandKind: "system_message", commandId: "CMD-1" }
    })).resolves.toBeUndefined();
    expect(finishMcpToolInvocation.mock.calls.at(-1)?.[0]).toMatchObject({
      actionReference: { resourceType: "message", resourceId: "MSG-1", commandKind: "system_message", commandId: "CMD-1" }
    });
    await expect(client.executeMessageCommand({
      taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-1", commandKind: "system_message", content: " notice ", requestId: "R1", traceId: "T1"
    })).resolves.toEqual({ resourceType: "message", resourceId: "MSG-1", commandKind: "system_message", commandId: "tool:" + "c".repeat(64) });
    await expect(client.projectTaskWorkflowState({
      taskId: "TASK-1", runId: "RUN-1", workflowId: "dipole-agent-task/TASK-1", workflowRunId: "temporal-run-1",
      workflowStatus: "waiting_approval", workflowRevision: 2
    }, identity)).resolves.toMatchObject({ workflowStatus: "waiting_approval", workflowRevision: 2 });
    await expect(client.listTaskWorkflowProjectionSnapshots("TASK-0", 100)).resolves.toEqual({
      tasks: [{
        taskId: "TASK-1", workflow: {
          workflowId: "dipole-agent-task/TASK-1", workflowRunId: "temporal-run-1",
          workflowStatus: "running", workflowRevision: 1
        }
      }, { taskId: "TASK-2" }],
      nextCursor: "TASK-2"
    });
    await expect(client.createArtifact({
      tenantId: "dipole", taskId: "TASK-1", runId: "RUN-1", artifactType: "report", version: 1,
      title: "Report", mediaType: "text/plain", content: Buffer.from("report"), metadata: { source: "G1" }
    })).resolves.toMatchObject({ artifactId: expect.stringMatching(/^[a-f0-9]{64}$/), contentSha256: createHash("sha256").update("report").digest("hex"), metadata: { source: "G1" } });
  });
});
