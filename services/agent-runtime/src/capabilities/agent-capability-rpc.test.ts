import { describe, expect, it, vi } from "vitest";
import { createHash } from "node:crypto";

import type { IAgentCapabilityServiceClient } from "../generated/dipole/agent/v1/agent.grpc-client.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import { AgentCapabilityRPCClient } from "./agent-capability-rpc.js";

const conversationReadContext = (overrides: Partial<ExecutionContext> = {}): ExecutionContext => ({
  tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId: "TASK-1", runId: "RUN-1",
  mode: "shadow", permissions: ["conversation.read"],
  resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["read"] }],
  approvedCapabilities: [], ...overrides
});

describe("AgentCapabilityRPCClient", () => {
  it("maps canonical group conversation ids to trusted RPC targets", async () => {
    const readConversation = vi.fn((input, metadata, _options, callback) => {
      expect(input).toMatchObject({ taskId: "TASK-1", runId: "RUN-1", targetId: "G123", limit: 20 });
      expect(input.context?.principalUserId).toBe("");
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, { found: true, reason: "", targetId: "G123", targetType: 2, messages: [] });
      return {};
    });
    const client = new AgentCapabilityRPCClient({ readConversation } as unknown as IAgentCapabilityServiceClient, "secret");

    await expect(client.readConversation(conversationReadContext({ requestId: "REQ-1", traceId: "TRACE-1" }), "group:G123", 20))
      .resolves.toMatchObject({ found: true, targetId: "G123", targetType: 2, messages: [] });
  });

  it("maps direct conversations relative to the authenticated principal", async () => {
    const readConversation = vi.fn((input, _metadata, _options, callback) => {
      expect(input.targetId).toBe("U200");
      callback(null, { found: false, reason: "not_found", targetId: "U200", targetType: 1, messages: [] });
      return {};
    });
    const client = new AgentCapabilityRPCClient({ readConversation } as unknown as IAgentCapabilityServiceClient, "secret");

    await expect(client.readConversation(conversationReadContext(), "direct:U200:U100", 1))
      .resolves.toEqual({ found: false, reason: "not_found", targetId: "U200", targetType: 1, messages: [] });
  });

  it("rejects invalid scopes, oversized responses, and conflicting RPC responses", async () => {
    const readConversation = vi.fn((_input, _metadata, _options, callback) => {
      callback(null, { found: true, reason: "", targetId: "U999", targetType: 1, messages: [] });
      return {};
    });
    const client = new AgentCapabilityRPCClient({ readConversation } as unknown as IAgentCapabilityServiceClient, "secret");
    const context = conversationReadContext();

    await expect(client.readConversation(context, "direct:U200:U300", 20)).rejects.toThrow("request is invalid");
    await expect(client.readConversation(context, "group:G123", 101)).rejects.toThrow("request is invalid");
    await expect(client.readConversation(context, "direct:U100:U200", 20)).rejects.toThrow("conflicting target");
    expect(readConversation).toHaveBeenCalledTimes(1);

    readConversation.mockImplementationOnce((_input, _metadata, _options, callback) => {
      callback(null, { found: false, reason: "not_found", targetId: "U200", targetType: 1, messages: Array.from({ length: 2 }, () => ({})) });
      return {};
    });
    await expect(client.readConversation(context, "direct:U100:U200", 1)).rejects.toThrow("too many messages");
  });

  it("resolves only an exact low-sensitive fresh readiness receipt", async () => {
    const profile = "b".repeat(64);
    const runtime = "a".repeat(64);
    const resolveFreshMcpReadinessEvidence = vi.fn((input, metadata, _options, callback) => {
      expect(input).toMatchObject({ tenantId: "dipole", profileBindingSha256: profile, runtimeBindingSha256: runtime });
      expect(input.context?.principalUserId).toBe("");
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, {
        found: true, evidenceId: "e".repeat(64), schemaVersion: "dipole.agent.external-mcp-readiness-evidence-record.v1",
        profileBindingSha256: profile, runtimeBindingSha256: runtime, contentSha256: "c".repeat(64), status: "recorded",
        collectedAtUnixMs: 1_000n, expiresAtUnixMs: 2_000n
      });
      return {};
    });
    const client = new AgentCapabilityRPCClient({ resolveFreshMcpReadinessEvidence } as unknown as IAgentCapabilityServiceClient, "secret");
    await expect(client.resolveFreshMcpReadinessEvidence("dipole", profile, runtime)).resolves.toMatchObject({
      evidenceId: "e".repeat(64), contentSha256: "c".repeat(64)
    });
  });

  it("accepts an empty readiness result and rejects contradictory empty evidence", async () => {
    const resolveFreshMcpReadinessEvidence = vi.fn((_input, _metadata, _options, callback) => {
      callback(null, {
        found: false, evidenceId: "", schemaVersion: "", profileBindingSha256: "", runtimeBindingSha256: "",
        contentSha256: "", status: "", collectedAtUnixMs: 0n, expiresAtUnixMs: 0n
      });
      return {};
    });
    const client = new AgentCapabilityRPCClient({ resolveFreshMcpReadinessEvidence } as unknown as IAgentCapabilityServiceClient, "secret");
    await expect(client.resolveFreshMcpReadinessEvidence("dipole", "b".repeat(64), "a".repeat(64))).resolves.toBeUndefined();
    resolveFreshMcpReadinessEvidence.mockImplementationOnce((_input, _metadata, _options, callback) => {
      callback(null, { found: false, evidenceId: "e".repeat(64) });
      return {};
    });
    await expect(client.resolveFreshMcpReadinessEvidence("dipole", "b".repeat(64), "a".repeat(64)))
      .rejects.toThrow("conflicting evidence");
  });

  it("publishes v2 readiness evidence with service-owned provenance and verifies the deterministic receipt", async () => {
    const evidence = {
      schemaVersion: "dipole.agent.external-mcp-readiness-evidence.v2" as const,
      bindingSha256: "a".repeat(64), profileBindingSha256: "b".repeat(64),
      startedAt: "2026-08-28T14:00:00.000Z", completedAt: "2026-08-28T14:00:03.000Z",
      preflightCheckedAt: "2026-08-28T14:00:01.000Z", connectivityCheckedAt: "2026-08-28T14:00:02.000Z",
      profileCount: 1, credentialCount: 1, caBundleCount: 1, toolCount: 2
    };
    const expiresAt = "2026-08-28T14:30:00.000Z";
    const publishMcpReadinessEvidence = vi.fn((input, metadata, _options, callback) => {
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      expect(input.context).toMatchObject({ principalUserId: "", requestId: "REQ-1", traceId: "TRACE-1", callerService: "dipole-agent" });
      expect(input).not.toHaveProperty("operatorId");
      expect(input).not.toHaveProperty("status");
      const content = Buffer.from(input.evidenceJson).toString("utf8");
      expect(JSON.parse(content)).toEqual(evidence);
      const contentSha256 = createHash("sha256").update(content).digest("hex");
      const evidenceId = createHash("sha256").update([
        "dipole.agent.external-mcp-readiness-evidence-record.v1", "dipole", evidence.profileBindingSha256,
        evidence.bindingSha256, contentSha256, "dipole-agent", "REQ-1", "TRACE-1", expiresAt
      ].join("\n")).digest("hex");
      callback(null, {
        evidenceId, schemaVersion: "dipole.agent.external-mcp-readiness-evidence-record.v1",
        profileBindingSha256: evidence.profileBindingSha256, runtimeBindingSha256: evidence.bindingSha256,
        contentSha256, status: "recorded", collectedAtUnixMs: BigInt(Date.parse(evidence.completedAt)),
        expiresAtUnixMs: BigInt(Date.parse(expiresAt)), created: true
      });
      return {};
    });
    const client = new AgentCapabilityRPCClient({ publishMcpReadinessEvidence } as unknown as IAgentCapabilityServiceClient, "secret");

    const receipt = await client.publishMcpReadinessEvidence("dipole", evidence, expiresAt, { requestId: "REQ-1", traceId: "TRACE-1" });

    expect(receipt).toMatchObject({ created: true, contentSha256: expect.stringMatching(/^[a-f0-9]{64}$/), expiresAt });
  });

  it("rejects stale input and conflicting readiness publication receipts", async () => {
    const evidence = {
      schemaVersion: "dipole.agent.external-mcp-readiness-evidence.v2" as const,
      bindingSha256: "a".repeat(64), profileBindingSha256: "b".repeat(64),
      startedAt: "2026-08-28T14:00:00.000Z", completedAt: "2026-08-28T14:00:03.000Z",
      preflightCheckedAt: "2026-08-28T14:00:01.000Z", connectivityCheckedAt: "2026-08-28T14:00:02.000Z",
      profileCount: 1, credentialCount: 1, caBundleCount: 1, toolCount: 2
    };
    const publishMcpReadinessEvidence = vi.fn((_input, _metadata, _options, callback) => {
      callback(null, {
        evidenceId: "f".repeat(64), schemaVersion: "dipole.agent.external-mcp-readiness-evidence-record.v1",
        profileBindingSha256: evidence.profileBindingSha256, runtimeBindingSha256: evidence.bindingSha256,
        contentSha256: "c".repeat(64), status: "recorded",
        collectedAtUnixMs: BigInt(Date.parse(evidence.completedAt)), expiresAtUnixMs: BigInt(Date.parse("2026-08-28T14:30:00.000Z")), created: false
      });
      return {};
    });
    const client = new AgentCapabilityRPCClient({ publishMcpReadinessEvidence } as unknown as IAgentCapabilityServiceClient, "secret");
    await expect(client.publishMcpReadinessEvidence("dipole", evidence, "2026-08-28T14:30:00.000Z"))
      .rejects.toThrow("conflicting evidence");
    await expect(client.publishMcpReadinessEvidence("dipole", evidence, "2026-08-28T16:00:00.000Z"))
      .rejects.toThrow("expiry is invalid");
    expect(publishMcpReadinessEvidence).toHaveBeenCalledTimes(1);
  });

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
        argumentsJson, argumentsSha256: createHash("sha256").update(argumentsJson).digest("hex"), startedAtUnixMs: 1_000n, status: "running"
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
    let beginMcpToolInvocationStatus = "running";
    const beginMcpToolInvocation = vi.fn((input, metadata, _options, callback) => {
      expect(input.context?.principalUserId).toBe("");
      expect(input).toMatchObject({ taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-1" });
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, { invocationId: input.invocationId, status: beginMcpToolInvocationStatus });
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
    const finishMcpToolInvocationFromRound = vi.fn((input, metadata, _options, callback) => {
      expect(input.context?.principalUserId).toBe("");
      expect(input).toMatchObject({ taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-EXT-1", roundId: "d".repeat(64) });
      expect(metadata.get("x-dipole-caller-service")).toEqual(["dipole-agent"]);
      callback(null, { invocationId: input.invocationId, status: "completed" });
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
    const client = new AgentCapabilityRPCClient({ admitRun, matchEventSubscriptions, listContextMemories, completeRun, finishRun, requestApproval, resolveApproval, consumeApproval, resolveApprovalGrant, listConversations, authorizeTaskControl, resolveMcpContext, beginMcpToolInvocation, resolveMcpToolCommand, claimMcpToolRound, finishMcpToolRound, finishMcpToolInvocation, finishMcpToolInvocationFromRound, executeMcpMessageCommand, projectTaskWorkflowState, listTaskWorkflowProjectionSnapshots, createArtifact } as unknown as IAgentCapabilityServiceClient, "secret");
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
    beginMcpToolInvocationStatus = "completed";
    await expect(client.beginMcpToolCommand({
      invocationId: "INV-1", taskId: "TASK-1", runId: "RUN-1", toolName: "calendar.read_event",
      capabilityId: "calendar.event.read", argumentsSha256: "a".repeat(64),
      profileId: "calendar-prod", serverId: "calendar.example", argumentsJson: `{"calendarId":"CAL-1"}`
    })).resolves.toEqual({ invocationId: "INV-1", status: "completed" });
    beginMcpToolInvocationStatus = "running";
    await expect(client.resolveMcpToolCommand("TASK-1", "RUN-1", "INV-EXT-1")).resolves.toMatchObject({
      profileId: "calendar-prod", serverId: "calendar.example", arguments: { calendarId: "CAL-1" }, startedAtUnixMs: 1_000, status: "running"
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
    await expect(client.finishMcpToolInvocationFromRound({
      taskId: "TASK-1", runId: "RUN-1", invocationId: "INV-EXT-1", roundId
    })).resolves.toEqual({ invocationId: "INV-EXT-1", status: "completed" });
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
