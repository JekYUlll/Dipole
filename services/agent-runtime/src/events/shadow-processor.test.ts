import { describe, expect, it, vi } from "vitest";
import type { Span } from "@opentelemetry/api";

import type { ExecutionContext } from "../runtime/execution-context.js";
import { CapabilityRegistry } from "../capabilities/registry.js";
import { ConversationListCapability } from "../capabilities/conversation-list.js";
import { ConversationReadCapability } from "../capabilities/conversation-read.js";
import { ConversationSearchCapability } from "../capabilities/conversation-search.js";
import { InMemoryEventLedger } from "./event-ledger.js";
import { ShadowEventProcessor, agentEventLedgerKey, agentRunId, agentTaskId, type AgentEvent } from "./shadow-processor.js";
import type { AgentTelemetry } from "../observability/agent-telemetry.js";

describe("ShadowEventProcessor", () => {
  it("nests a Run span under the Task processing boundary", async () => {
    const names: string[] = [];
    const telemetry = {
      withSpan: vi.fn(async (name: string, _context: unknown, operation: (span: Span) => Promise<unknown>) => {
        names.push(name);
        return operation({ setAttribute: vi.fn() } as unknown as Span);
      })
    };
    const processor = new ShadowEventProcessor(
      { plan: async () => ({ summary: "observe", steps: [] }) },
      { append: async () => undefined }, new InMemoryEventLedger(),
      undefined, undefined, undefined, 60_000, undefined, telemetry as unknown as Pick<AgentTelemetry, "withSpan">
    );

    await processor.process({
      eventId: "E-TRACE", eventType: "message.direct.created", aggregateId: "M-TRACE",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: {}
    }, { tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI" });

    expect(names).toEqual(["agent.task", "agent.run"]);
  });

  it("suppresses events from the same Agent causal chain before claiming or dispatching", async () => {
    const plan = vi.fn();
    const append = vi.fn();
    const claim = vi.fn();
    const dispatch = vi.fn();
    const processor = new ShadowEventProcessor(
      { plan }, { append }, { claim, complete: vi.fn(), release: vi.fn() },
      undefined, undefined, undefined, 60_000, { dispatch }
    );
    const event = {
      eventId: "E-LOOP", eventType: "message.direct.created", aggregateId: "M-LOOP",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: {},
      lineage: {
        origin: { type: "agent" as const, id: "UAI" },
        causationEventId: "E-REQUEST",
        agentTaskId: "TASK-ORIGIN"
      }
    };

    await expect(processor.process(event, {
      tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI"
    })).resolves.toEqual({ outcome: "suppressed", taskId: expect.stringMatching(/^task:/) });
    expect(claim).not.toHaveBeenCalled();
    expect(dispatch).not.toHaveBeenCalled();
    expect(plan).not.toHaveBeenCalled();
    expect(append).not.toHaveBeenCalled();
  });

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

  it("derives subscription-specific Task and ledger identities", () => {
    const direct = agentTaskId({ tenantId: "dipole", agentUuid: "UAI", triggerType: "message.direct.created", triggerRef: "M100" });
    const first = agentTaskId({ tenantId: "dipole", agentUuid: "UAI", triggerType: "message.direct.created", triggerRef: "M100", subscriptionId: "SUB-1" });
    const second = agentTaskId({ tenantId: "dipole", agentUuid: "UAI", triggerType: "message.direct.created", triggerRef: "M100", subscriptionId: "SUB-2" });
    expect(new Set([direct, first, second]).size).toBe(3);
    expect(agentEventLedgerKey({ eventId: "E1" })).toBe("E1");
    expect(agentEventLedgerKey({ eventId: "E1", subscriptionId: "SUB-1" })).not.toBe(agentEventLedgerKey({ eventId: "E1", subscriptionId: "SUB-2" }));
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
      recordAuthorization: vi.fn(async () => undefined),
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

  it("rejects a direct conversation read before invoking the remote capability", async () => {
    const registry = new CapabilityRegistry();
    const readConversation = vi.fn(async () => ({ found: true, reason: "", targetId: "G1", targetType: 2, messages: [] }));
    registry.register(new ConversationReadCapability({ readConversation }));
    const trajectory = {
      append: vi.fn(async () => undefined),
      claimStep: vi.fn(async () => ({ outcome: "claimed" as const, token: "TOKEN-DIRECT" })),
      recordAuthorization: vi.fn(async () => undefined),
      completeStep: vi.fn(async () => undefined),
      failStep: vi.fn(async () => undefined)
    };
    const processor = new ShadowEventProcessor(
      { plan: async () => ({ summary: "unsafe", steps: [{ capabilityId: "conversation.read", input: { conversationId: "group:G1" } }] }) },
      trajectory, new InMemoryEventLedger(), undefined, registry, trajectory
    );

    await expect(processor.process({
      eventId: "E-DIRECT", eventType: "message.direct.created", aggregateId: "M-DIRECT",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: {}
    }, { tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI" })).rejects.toThrow(/trusted conversation discovery marker/);
    expect(readConversation).not.toHaveBeenCalled();
    expect(trajectory.recordAuthorization).not.toHaveBeenCalled();
    expect(trajectory.failStep).toHaveBeenCalledOnce();
  });

  it("records an empty discovery and skips its dependent read", async () => {
    const registry = new CapabilityRegistry();
    const listConversations = vi.fn(async () => []);
    const readConversation = vi.fn(async () => ({ found: true, reason: "", targetId: "G1", targetType: 2, messages: [] }));
    registry.register(new ConversationListCapability({ listConversations }));
    registry.register(new ConversationReadCapability({ readConversation }));
    let claimCount = 0;
    const trajectory = {
      append: vi.fn(async () => undefined),
      claimStep: vi.fn(async () => ({ outcome: "claimed" as const, token: `TOKEN-${++claimCount}` })),
      recordAuthorization: vi.fn(async () => undefined),
      completeStep: vi.fn(async () => undefined),
      failStep: vi.fn(async () => undefined)
    };
    const processor = new ShadowEventProcessor(
      { plan: async () => ({ summary: "read newest", steps: [
        { capabilityId: "conversation.list", input: { limit: 20 } },
        { capabilityId: "conversation.read", input: { conversationId: "$discovered.previous" } }
      ] }) },
      trajectory, new InMemoryEventLedger(), undefined, registry, trajectory
    );

    await expect(processor.process({
      eventId: "E-EMPTY-DISCOVERY", eventType: "message.direct.created", aggregateId: "M-EMPTY-DISCOVERY",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: {}
    }, { tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI" })).resolves.toMatchObject({ outcome: "recorded" });

    expect(listConversations).toHaveBeenCalledOnce();
    expect(readConversation).not.toHaveBeenCalled();
    expect(trajectory.recordAuthorization).toHaveBeenCalledOnce();
    expect(trajectory.completeStep).toHaveBeenNthCalledWith(1, expect.any(String), 1, "TOKEN-1", []);
    expect(trajectory.completeStep).toHaveBeenNthCalledWith(2, expect.any(String), 2, "TOKEN-2", {
      status: "skipped", reason: "no_discovered_conversation"
    });
    expect(trajectory.failStep).not.toHaveBeenCalled();
  });

  it("executes a retrieval Step only when the composed context grants search", async () => {
    const registry = new CapabilityRegistry();
    const searchConversations = vi.fn(async () => []);
    registry.register(new ConversationSearchCapability({ searchConversations }));
    const trajectory = {
      append: vi.fn(async () => undefined),
      claimStep: vi.fn(async () => ({ outcome: "claimed" as const, token: "TOKEN-SEARCH" })),
      recordAuthorization: vi.fn(async () => undefined),
      completeStep: vi.fn(async () => undefined),
      failStep: vi.fn(async () => undefined)
    };
    const processor = new ShadowEventProcessor(
      { plan: async () => ({ summary: "search", steps: [{ capabilityId: "conversation.search", input: { query: "migration", limit: 5 } }] }) },
      trajectory, new InMemoryEventLedger(), undefined, registry, trajectory,
      60_000, undefined, undefined, ["conversation.list", "conversation.read", "conversation.search"]
    );

    await processor.process({
      eventId: "E-SEARCH", eventType: "message.direct.created", aggregateId: "M-SEARCH",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: {}
    }, { tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI" });

    expect(searchConversations).toHaveBeenCalledWith(expect.objectContaining({
      permissions: ["conversation.list", "conversation.read", "conversation.search"]
    }), "migration", 5);
    expect(trajectory.completeStep).toHaveBeenCalledWith(expect.stringMatching(/^task:/), 1, "TOKEN-SEARCH", []);
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

  it("hands an exact event binding to Temporal and skips inline planning", async () => {
    const plan = vi.fn();
    const dispatch = vi.fn(async () => undefined);
    const setAttribute = vi.fn();
    const telemetry = {
      withSpan: vi.fn(async (_name: string, _context: unknown, operation: (span: Span) => Promise<unknown>) => {
        return operation({ setAttribute } as unknown as Span);
      })
    };
    const processor = new ShadowEventProcessor(
      { plan }, { append: vi.fn() }, new InMemoryEventLedger(),
      undefined, undefined, undefined, 60_000, { dispatch }, telemetry as unknown as Pick<AgentTelemetry, "withSpan">
    );
    const event = {
      eventId: "E-DISPATCH", eventType: "message.direct.created", aggregateId: "M-DISPATCH",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: {}
    };
    const identity = { tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", requestId: "R1", traceId: "T1" };

    const result = await processor.process(event, identity);

    expect(dispatch).toHaveBeenCalledWith(event, identity, result.taskId);
    expect(plan).not.toHaveBeenCalled();
    expect(setAttribute).toHaveBeenCalledWith("dipole.agent.task.outcome", "recorded");
    await expect(processor.process(event, identity)).resolves.toMatchObject({ outcome: "duplicate" });
  });

  it("releases the Event claim when Temporal start fails", async () => {
    const dispatch = vi.fn()
      .mockRejectedValueOnce(new Error("Temporal unavailable"))
      .mockResolvedValueOnce(undefined);
    const processor = new ShadowEventProcessor(
      { plan: vi.fn() }, { append: vi.fn() }, new InMemoryEventLedger(),
      undefined, undefined, undefined, 60_000, { dispatch }
    );
    const event = {
      eventId: "E-DISPATCH-RETRY", eventType: "message.direct.created", aggregateId: "M-DISPATCH-RETRY",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: {}
    };
    const identity = { tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI" };

    await expect(processor.process(event, identity)).rejects.toThrow("Temporal unavailable");
    await expect(processor.process(event, identity)).resolves.toMatchObject({ outcome: "recorded" });
    expect(dispatch).toHaveBeenCalledTimes(2);
  });
});
