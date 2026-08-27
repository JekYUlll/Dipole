import { describe, expect, it, vi } from "vitest";

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
    const client = new AgentCapabilityRPCClient({ admitRun, completeRun, finishRun, requestApproval, resolveApproval, listConversations, authorizeTaskControl, projectTaskWorkflowState } as unknown as IAgentCapabilityServiceClient, "secret");
    const identity = { tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", requestId: "R1", traceId: "T1" };
    const event = { eventId: "E1", eventType: "message.direct.created", aggregateId: "M1", occurredAt: "2026-08-27T08:00:00.000Z", payload: {} };

    await expect(client.admit(event, identity)).resolves.toEqual({ taskId: "TASK-1", runId: "RUN-1", runStatus: "running" });
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
    await expect(client.authorizeTaskControl("TASK-1", "U100", identity)).resolves.toEqual({
      taskId: "TASK-1", taskStatus: "running",
      workflow: {
        taskId: "TASK-1", workflowId: "dipole-agent-task/TASK-1", workflowRunId: "temporal-run-1",
        workflowStatus: "waiting_approval", workflowRevision: 2
      }
    });
    await expect(client.projectTaskWorkflowState({
      taskId: "TASK-1", runId: "RUN-1", workflowId: "dipole-agent-task/TASK-1", workflowRunId: "temporal-run-1",
      workflowStatus: "waiting_approval", workflowRevision: 2
    }, identity)).resolves.toMatchObject({ workflowStatus: "waiting_approval", workflowRevision: 2 });
  });
});
