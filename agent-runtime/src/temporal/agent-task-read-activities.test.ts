import { describe, expect, it, vi } from "vitest";

import { CapabilityRegistry } from "../capabilities/registry.js";
import { ConversationListCapability } from "../capabilities/conversation-list.js";
import { agentRunId, agentTaskId, type AgentEvent } from "../events/shadow-processor.js";
import { ModelShadowPlanner } from "../models/model-shadow-planner.js";
import type { ModelRouter } from "../models/model-router.js";
import { createTemporalReadStepActivities } from "./agent-task-read-activities.js";

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
    const activities = createTemporalReadStepActivities({ planner, audit: trajectory, registry, trajectory, artifacts, stepLeaseMs: 60_000 });

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
