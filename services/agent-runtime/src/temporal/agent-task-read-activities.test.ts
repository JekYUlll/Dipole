import { describe, expect, it, vi } from "vitest";
import type { Span } from "@opentelemetry/api";
import { createHash } from "node:crypto";

import { CapabilityRegistry } from "../capabilities/registry.js";
import { ConversationListCapability } from "../capabilities/conversation-list.js";
import { agentRunId, agentTaskId, type AgentEvent } from "../events/shadow-processor.js";
import { ModelShadowPlanner } from "../models/model-shadow-planner.js";
import type { ModelRouter } from "../models/model-router.js";
import { createTemporalReadStepActivities } from "./agent-task-read-activities.js";
import type { AgentTelemetry } from "../observability/agent-telemetry.js";

describe("Temporal read Step Activities", () => {
  it("compiles, plans, persists, and executes a read-only Step under the exact Task binding", async () => {
    const event: AgentEvent = {
      eventId: "E-TEMPORAL-READ", eventType: "message.direct.created", aggregateId: "M-TEMPORAL-READ",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: { content: "untrusted message" }
    };
    const taskId = agentTaskId({ tenantId: "dipole", agentUuid: "UAI", triggerType: event.eventType, triggerRef: event.aggregateId });
    const generate = vi.fn(async ({ prompt }: { prompt: string }) => {
      expect(prompt).toContain("untrusted message");
      expect(prompt).toContain("conversation.list");
      return {
        output: { summary: "list conversations", steps: [{ capabilityId: "conversation.list", input: { limit: 10 } }] },
        route: "gateway/primary", attempts: 1, usage: { inputTokens: 20, outputTokens: 8 }
      };
    });
    const planner = new ModelShadowPlanner({ generate } as unknown as ModelRouter, ["conversation.list"]);
    const conversation = {
      conversationKey: "group:G1", targetId: "G1", targetType: 2,
      lastMessageId: "M1", lastMessageSeq: "1", lastMessagePreview: "hello",
      lastMessageAtUnixMs: "1787817600000", readSeq: "0", unreadCount: 1
    };
    const listConversations = vi.fn(async () => [conversation]);
    const registry = new CapabilityRegistry();
    registry.register(new ConversationListCapability({ listConversations }));
    const trajectory = {
      append: vi.fn(async () => undefined),
      claimStep: vi.fn(async () => ({ outcome: "claimed" as const, token: "TOKEN-1" })),
      completeStep: vi.fn(async () => undefined),
      failStep: vi.fn(async () => undefined)
    };
    const artifacts = { createArtifact: vi.fn(async () => ({
      schemaVersion: "dipole.agent.artifact.v1" as const, artifactId: "a".repeat(64), taskId,
      runId: agentRunId(taskId), artifactType: "conversation_digest", version: 1,
      title: "Conversation digest", mediaType: "text/markdown", contentSha256: "b".repeat(64),
      sizeBytes: 10, metadata: {}
    })) };
    const spanNames: string[] = [];
    const telemetry = recordingTelemetry(spanNames);
    const activities = createTemporalReadStepActivities({ planner, audit: trajectory, registry, trajectory, artifacts, telemetry, stepLeaseMs: 60_000 });

    await expect(activities.executeAgentTaskStep({
      taskId, runId: agentRunId(taskId), goal: "observe", step: 0, shadowEvent: event,
      admission: {
        tenantId: "dipole", principalUserId: "U100", agentId: "UAI",
        triggerType: event.eventType, triggerRef: event.aggregateId, eventId: event.eventId
      }
    })).resolves.toEqual({ kind: "complete", output: { summary: "list conversations", stepCount: 1, artifactId: "a".repeat(64), artifactVersion: 1 } });

    expect(trajectory.append).toHaveBeenCalledOnce();
    expect(listConversations).toHaveBeenCalledOnce();
    expect(trajectory.completeStep).toHaveBeenCalledWith(taskId, 1, "TOKEN-1", [conversation]);
    expect(artifacts.createArtifact).toHaveBeenCalledWith(expect.objectContaining({
      taskId, runId: agentRunId(taskId), artifactType: "conversation_digest", version: 1,
      metadata: { event_id: event.eventId, event_type: event.eventType, step_count: 1 }
    }));
    expect(spanNames).toEqual(["agent.run", "agent.tool.call", "agent.artifact.create"]);
  });

  it("rejects event or Run drift before planning", async () => {
    const planner = { plan: vi.fn() };
    const registry = new CapabilityRegistry();
    const trajectory = {
      append: vi.fn(), claimStep: vi.fn(), completeStep: vi.fn(), failStep: vi.fn()
    };
    const activities = createTemporalReadStepActivities({ planner, audit: trajectory, registry, trajectory, stepLeaseMs: 60_000 });

    await expect(activities.executeAgentTaskStep({
      taskId: "task:forged", runId: "run:forged", goal: "observe", step: 0,
      shadowEvent: { eventId: "E1", eventType: "message.direct.created", aggregateId: "M1", occurredAt: "2026-08-27T08:00:00.000Z", payload: {} },
      admission: { tenantId: "dipole", principalUserId: "U100", agentId: "UAI", triggerType: "message.direct.created", triggerRef: "M1", eventId: "E1" }
    })).rejects.toThrow(/binding mismatch/);
    expect(planner.plan).not.toHaveBeenCalled();
  });

  it("uses the Core-owned Context for an active read Step", async () => {
    const event: AgentEvent = {
      eventId: "E-ACTIVE-READ", eventType: "message.direct.created", aggregateId: "M-ACTIVE-READ",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: { content: "active read" }
    };
    const taskId = agentTaskId({ tenantId: "dipole", agentUuid: "UAI", triggerType: event.eventType, triggerRef: event.aggregateId });
    const runId = agentRunId(taskId, "dipole-agent", "active");
    const contextResolver = {
      resolveMcpContext: vi.fn(async () => ({
        tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId, runId, mode: "active" as const,
        permissions: ["conversation.list", "conversation.read"],
        resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["read", "list"] }],
        approvedCapabilities: [], eventId: event.eventId
      }))
    };
    const activities = createTemporalReadStepActivities({
      planner: { plan: async () => ({ summary: "active observe", steps: [] }) },
      audit: { append: vi.fn(async () => undefined) },
      registry: new CapabilityRegistry(), trajectory: { append: vi.fn(async () => undefined), claimStep: vi.fn(async () => ({ outcome: "claimed" as const, token: "TOKEN-ACTIVE" })), completeStep: vi.fn(async () => undefined), failStep: vi.fn(async () => undefined) },
      runtimeMode: "active", contextResolver, stepLeaseMs: 60_000
    });

    await expect(activities.executeAgentTaskStep({
      taskId, runId, goal: "observe", step: 0, shadowEvent: event,
      admission: { tenantId: "dipole", principalUserId: "U100", agentId: "UAI", triggerType: event.eventType, triggerRef: event.aggregateId, eventId: event.eventId }
    })).resolves.toEqual({ kind: "complete", output: { summary: "active observe", stepCount: 0 } });
    expect(contextResolver.resolveMcpContext).toHaveBeenCalledWith(taskId, runId, "U100", {});
  });

  it("writes one audited assistant reply for an active direct-message Task", async () => {
    const event: AgentEvent = {
      eventId: "E-ACTIVE-REPLY", eventType: "message.direct.created", aggregateId: "M-ACTIVE-REPLY",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: { content: "hello", conversation_key: "direct:U100:UAI" }
    };
    const taskId = agentTaskId({ tenantId: "dipole", agentUuid: "UAI", triggerType: event.eventType, triggerRef: event.aggregateId });
    const runId = agentRunId(taskId, "dipole-agent", "active");
    const context = {
      tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId, runId, mode: "active" as const,
      permissions: ["message.write"], resourceScopes: [{ resourceType: "conversation", resourceId: "*", actions: ["write"] }],
      approvedCapabilities: [], eventId: event.eventId
    };
    const replyWriter = {
      begin: vi.fn(async () => undefined),
      finishToolInvocation: vi.fn(async () => undefined),
      executeMessageCommand: vi.fn(async () => ({ resourceType: "message" as const, resourceId: "MSG-1", commandKind: "assistant_reply" as const, commandId: "CMD-1" }))
    };
    const activities = createTemporalReadStepActivities({
      planner: { plan: async () => ({ summary: "Hello from Dipole", steps: [] }) },
      audit: { append: vi.fn(async () => undefined) }, registry: new CapabilityRegistry(),
      trajectory: { append: vi.fn(async () => undefined), claimStep: vi.fn(async () => ({ outcome: "claimed" as const, token: "TOKEN-ACTIVE" })), completeStep: vi.fn(async () => undefined), failStep: vi.fn(async () => undefined) },
      runtimeMode: "active", contextResolver: { resolveMcpContext: vi.fn(async () => context) }, replyWriter, stepLeaseMs: 60_000
    });

    await expect(activities.executeAgentTaskStep({
      taskId, runId, goal: "reply", step: 0, shadowEvent: event,
      admission: { tenantId: "dipole", principalUserId: "U100", agentId: "UAI", triggerType: event.eventType, triggerRef: event.aggregateId, eventId: event.eventId }
    })).resolves.toEqual({ kind: "complete", output: { summary: "Hello from Dipole", stepCount: 0, replyMessageId: "MSG-1" } });
    expect(replyWriter.begin).toHaveBeenCalledWith(expect.objectContaining({
      taskId, runId, capabilityId: "message.assistant_reply.send"
    }));
    expect(replyWriter.executeMessageCommand).toHaveBeenCalledWith(expect.objectContaining({
      taskId, runId, commandKind: "assistant_reply", conversationKey: "direct:U100:UAI", content: "Hello from Dipole"
    }));
    expect(replyWriter.finishToolInvocation).toHaveBeenCalledWith(expect.objectContaining({ status: "completed" }));
  });

  it("waits for approval before delivering an explicit system message", async () => {
    const event: AgentEvent = {
      eventId: "E-APPROVAL", eventType: "message.direct.created", aggregateId: "M-APPROVAL",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: { content: "/system Deployment starts at 18:00", conversation_key: "direct:U100:UAI" }
    };
    const taskId = agentTaskId({ tenantId: "dipole", agentUuid: "UAI", triggerType: event.eventType, triggerRef: event.aggregateId });
    const runId = agentRunId(taskId, "dipole-agent", "active");
    const context = {
      tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId, runId, mode: "active" as const,
      permissions: ["message.write"], resourceScopes: [{ resourceType: "conversation", resourceId: "direct:U100:UAI", actions: ["write"] }],
      approvedCapabilities: ["message.system.send"] as "message.system.send"[], eventId: event.eventId
    };
    const approvalWriter = {
      begin: vi.fn(async () => undefined),
      finishToolInvocation: vi.fn(async () => undefined),
      consumeApproval: vi.fn(async () => undefined),
      resolveApprovalGrant: vi.fn(async (_taskId: string, _runId: string, capabilityId: string, resourceScope: { resourceType: string; resourceId: string; actions: string[] }, argumentsSha256: string) => ({
        approvalId: "APR-1", capabilityId, resourceScope,
        scopeSha256: createHash("sha256").update(["dipole.agent.scope.v1", resourceScope.resourceType, resourceScope.resourceId, ...resourceScope.actions].join("\n"), "utf8").digest("hex"),
        argumentsSha256, nonceSha256: "1".repeat(64), expiresAtUnixMs: Date.now() + 60_000
      })),
      executeMessageCommand: vi.fn(async () => ({ resourceType: "message" as const, resourceId: "MSG-SYSTEM-1", commandKind: "system_message" as const, commandId: "CMD-SYSTEM-1" }))
    };
    const activities = createTemporalReadStepActivities({
      planner: { plan: vi.fn(async () => ({ summary: "unused", steps: [] })) }, audit: { append: vi.fn(async () => undefined) }, registry: new CapabilityRegistry(),
      trajectory: { append: vi.fn(async () => undefined), claimStep: vi.fn(async () => ({ outcome: "claimed" as const, token: "TOKEN" })), completeStep: vi.fn(async () => undefined), failStep: vi.fn(async () => undefined) },
      runtimeMode: "active", contextResolver: { resolveMcpContext: vi.fn(async () => context) }, approvalWriter, stepLeaseMs: 60_000
    });

    const initial = await activities.executeAgentTaskStep({
      taskId, runId, goal: "notify", step: 0, shadowEvent: event,
      admission: { tenantId: "dipole", principalUserId: "U100", agentId: "UAI", triggerType: event.eventType, triggerRef: event.aggregateId, eventId: event.eventId }
    });
    expect(initial).toMatchObject({ kind: "wait_approval", approval: { capabilityId: "message.system.send" } });
    expect(approvalWriter.executeMessageCommand).not.toHaveBeenCalled();

    await expect(activities.executeAgentTaskStep({
      taskId, runId, goal: "notify", step: 1, resume: { kind: "approval", requestId: (initial as { requestId: string }).requestId, approvalId: (initial as { approval: { approvalId: string } }).approval.approvalId, decision: "approved" }, shadowEvent: event,
      admission: { tenantId: "dipole", principalUserId: "U100", agentId: "UAI", triggerType: event.eventType, triggerRef: event.aggregateId, eventId: event.eventId }
    })).resolves.toMatchObject({ kind: "complete", output: { summary: "Approved system message delivered" } });
    expect(approvalWriter.consumeApproval).toHaveBeenCalledOnce();
    expect(approvalWriter.executeMessageCommand).toHaveBeenCalledOnce();
  });

  it("writes a group reply to the triggering conversation", async () => {
    const event: AgentEvent = {
      eventId: "E-GROUP-REPLY", eventType: "message.group.created", aggregateId: "M-GROUP-REPLY",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: { content: "@AI summarize", conversation_key: "group:G100" }
    };
    const taskId = agentTaskId({ tenantId: "dipole", agentUuid: "UAI", triggerType: event.eventType, triggerRef: event.aggregateId });
    const runId = agentRunId(taskId, "dipole-agent", "active");
    const replyWriter = {
      begin: vi.fn(async () => undefined), finishToolInvocation: vi.fn(async () => undefined),
      executeMessageCommand: vi.fn(async () => ({ resourceType: "message" as const, resourceId: "MSG-GROUP-1", commandKind: "group_reply" as const, commandId: "CMD-GROUP-1" }))
    };
    const activities = createTemporalReadStepActivities({
      planner: { plan: async () => ({ summary: "Group summary", steps: [] }) }, audit: { append: vi.fn(async () => undefined) }, registry: new CapabilityRegistry(),
      trajectory: { append: vi.fn(async () => undefined), claimStep: vi.fn(async () => ({ outcome: "claimed" as const, token: "TOKEN-GROUP" })), completeStep: vi.fn(async () => undefined), failStep: vi.fn(async () => undefined) },
      runtimeMode: "active", contextResolver: { resolveMcpContext: vi.fn(async () => ({
        tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId, runId, mode: "active" as const,
        permissions: ["message.write"], resourceScopes: [{ resourceType: "conversation", resourceId: "group:G100", actions: ["write"] }], approvedCapabilities: [], eventId: event.eventId
      })) }, replyWriter, stepLeaseMs: 60_000
    });

    await expect(activities.executeAgentTaskStep({
      taskId, runId, goal: "reply", step: 0, shadowEvent: event,
      admission: { tenantId: "dipole", principalUserId: "U100", agentId: "UAI", triggerType: event.eventType, triggerRef: event.aggregateId, eventId: event.eventId }
    })).resolves.toEqual({ kind: "complete", output: { summary: "Group summary", stepCount: 0, replyMessageId: "MSG-GROUP-1" } });
    expect(replyWriter.begin).toHaveBeenCalledWith(expect.objectContaining({ capabilityId: "message.group_reply.send" }));
    expect(replyWriter.executeMessageCommand).toHaveBeenCalledWith(expect.objectContaining({ commandKind: "group_reply", conversationKey: "group:G100" }));
  });

  it("waits for a crashed Step lease and accepts its completed replay", async () => {
    const event: AgentEvent = {
      eventId: "E-BUSY", eventType: "message.direct.created", aggregateId: "M-BUSY",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: {}
    };
    const taskId = agentTaskId({ tenantId: "dipole", agentUuid: "UAI", triggerType: event.eventType, triggerRef: event.aggregateId });
    const registry = new CapabilityRegistry();
    const claimStep = vi.fn()
      .mockResolvedValueOnce({ outcome: "busy" as const })
      .mockResolvedValueOnce({ outcome: "completed" as const });
    const trajectory = {
      append: vi.fn(async () => undefined), claimStep,
      completeStep: vi.fn(async () => undefined), failStep: vi.fn(async () => undefined)
    };
    const activities = createTemporalReadStepActivities({
      planner: { plan: async () => ({ summary: "recovered", steps: [{ capabilityId: "conversation.list", input: {} }] }) },
      audit: trajectory, registry, trajectory, stepLeaseMs: 1000,
      busyStepRetry: { intervalMs: 1, maxWaitMs: 5 }
    });

    await expect(activities.executeAgentTaskStep({
      taskId, runId: agentRunId(taskId), goal: "observe", step: 0, shadowEvent: event,
      admission: { tenantId: "dipole", principalUserId: "U100", agentId: "UAI", triggerType: event.eventType, triggerRef: event.aggregateId, eventId: event.eventId }
    })).resolves.toEqual({ kind: "complete", output: { summary: "recovered", stepCount: 1 } });
    expect(claimStep).toHaveBeenCalledTimes(2);
    expect(trajectory.completeStep).not.toHaveBeenCalled();
  });
});

function recordingTelemetry(names: string[]): Pick<AgentTelemetry, "withSpan"> {
  return {
    withSpan: vi.fn(async (name: string, _context: unknown, operation: (span: Span) => Promise<unknown>) => {
      names.push(name);
      return operation({ setAttribute: vi.fn() } as unknown as Span);
    })
  } as unknown as Pick<AgentTelemetry, "withSpan">;
}
