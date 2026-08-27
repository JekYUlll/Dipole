import { describe, expect, it, vi } from "vitest";

import type { ExecutionContext } from "../runtime/execution-context.js";
import { CapabilityRegistry } from "../capabilities/registry.js";
import { ConversationListCapability } from "../capabilities/conversation-list.js";
import { InMemoryEventLedger } from "./event-ledger.js";
import { ShadowEventProcessor, agentRunId, agentTaskId, type AgentEvent } from "./shadow-processor.js";

describe("ShadowEventProcessor", () => {
  it("uses the Go-compatible Task ID and records each event once", async () => {
    expect(agentTaskId({
      tenantId: "dipole", agentUuid: "UAI", triggerType: "message.direct.created", triggerRef: "M100"
    })).toBe("task:e47647aaf491da8a27072ed94d6b69b87a025a1e211000cbef6a9aeb458");
    expect(agentRunId("task:e47647aaf491da8a27072ed94d6b69b87a025a1e211000cbef6a9aeb458")).toBe("run:fe813966ff90ac9c0a32e5d7b6a55dadbba657f436ad3a3765e9466aba0b");

    const plan = vi.fn(async (_event: AgentEvent, _context: ExecutionContext) => ({
      summary: "read only plan", steps: [{ capabilityId: "conversation.list", input: { limit: 20 } }]
    }));
    const append = vi.fn(async () => undefined);
    const processor = new ShadowEventProcessor({ plan }, { append }, new InMemoryEventLedger());
    const event = {
      eventId: "E1",
      eventType: "message.direct.created",
      aggregateId: "M100",
      occurredAt: "2026-08-27T08:00:00.000Z",
      payload: { sender_uuid: "U100" }
    };
    const identity = { tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI" };

    await expect(processor.process(event, identity)).resolves.toEqual({ outcome: "recorded", taskId: expect.stringMatching(/^task:/) });
    await expect(processor.process(event, identity)).resolves.toEqual({ outcome: "duplicate", taskId: expect.stringMatching(/^task:/) });
    expect(plan).toHaveBeenCalledTimes(1);
    expect(plan.mock.calls[0]?.[1].mode).toBe("shadow");
    expect(plan.mock.calls[0]?.[1].runId).toMatch(/^run:/);
    expect(append).toHaveBeenCalledTimes(1);
  });

  it("executes an admitted read-only Step and persists its terminal result", async () => {
    const registry = new CapabilityRegistry();
    const conversation = {
      conversationKey: "group:G1", targetId: "G1", targetType: 2,
      lastMessageId: "M1", lastMessageSeq: "1", lastMessagePreview: "hello",
      lastMessageAtUnixMs: "1787817600000", readSeq: "0", unreadCount: 1
    };
    const listConversations = vi.fn(async () => [conversation]);
    registry.register(new ConversationListCapability({ listConversations }));
    const trajectory = {
      append: vi.fn(async () => undefined),
      claimStep: vi.fn(async () => ({ outcome: "claimed" as const, token: "TOKEN-1" })),
      completeStep: vi.fn(async () => undefined),
      failStep: vi.fn(async () => undefined)
    };
    const completeRun = vi.fn(async () => undefined);
    const processor = new ShadowEventProcessor({
      plan: async () => ({ summary: "list", steps: [{ capabilityId: "conversation.list", input: { limit: 20 } }] })
    }, trajectory, new InMemoryEventLedger(), {
      admit: async (_event, identity) => {
        const taskId = agentTaskId({ tenantId: identity.tenantId, agentUuid: identity.agentUuid, triggerType: "message.direct.created", triggerRef: "M-STEP" });
        return { taskId, runId: agentRunId(taskId), runStatus: "running" };
      },
      complete: completeRun
    }, registry, trajectory);

    await processor.process({
      eventId: "E-STEP", eventType: "message.direct.created", aggregateId: "M-STEP",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: {}
    }, { tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI" });

    expect(listConversations).toHaveBeenCalledOnce();
    expect(trajectory.completeStep).toHaveBeenCalledWith(expect.stringMatching(/^task:/), 1, "TOKEN-1", [conversation]);
    expect(trajectory.failStep).not.toHaveBeenCalled();
    expect(completeRun).toHaveBeenCalledWith(expect.stringMatching(/^task:/), expect.stringMatching(/^run:/), expect.objectContaining({ mode: "shadow" }));
  });

  it("releases its claim when planning fails so Kafka retry can resume", async () => {
    const ledger = new InMemoryEventLedger();
    const plan = vi.fn()
      .mockRejectedValueOnce(new Error("temporary model failure"))
      .mockResolvedValueOnce({ summary: "retry", steps: [] });
    const processor = new ShadowEventProcessor({ plan }, { append: async () => undefined }, ledger);
    const event = {
      eventId: "E-RETRY", eventType: "message.direct.created", aggregateId: "M-RETRY",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: { sender_uuid: "U100" }
    };
    const identity = { tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI" };

    await expect(processor.process(event, identity)).rejects.toThrow("temporary model failure");
    await expect(processor.process(event, identity)).resolves.toMatchObject({ outcome: "recorded" });
    expect(plan).toHaveBeenCalledTimes(2);
  });
});
