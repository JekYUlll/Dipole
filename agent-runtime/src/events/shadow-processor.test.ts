import { describe, expect, it, vi } from "vitest";

import type { ExecutionContext } from "../runtime/execution-context.js";
import { ShadowEventProcessor, agentTaskId, type AgentEvent } from "./shadow-processor.js";

describe("ShadowEventProcessor", () => {
  it("uses the Go-compatible Task ID and records each event once", async () => {
    expect(agentTaskId({
      tenantId: "dipole", agentUuid: "UAI", triggerType: "message.direct.created", triggerRef: "M100"
    })).toBe("task:e47647aaf491da8a27072ed94d6b69b87a025a1e211000cbef6a9aeb458");

    const plan = vi.fn(async (_event: AgentEvent, _context: ExecutionContext) => ({ summary: "read only plan", capabilityIds: ["conversation.read"] }));
    const append = vi.fn(async () => undefined);
    const processor = new ShadowEventProcessor({ plan }, { append });
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
    expect(append).toHaveBeenCalledTimes(1);
  });
});
