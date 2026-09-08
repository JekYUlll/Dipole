import { describe, expect, it, vi } from "vitest";

import { createAgentEventLedgerLifecycleActivities } from "./agent-event-ledger-activities.js";

describe("Agent Event Ledger Temporal activities", () => {
  const claim = { eventId: "E1", taskId: "TASK-1", token: "TOKEN-1" };

  it("completes the event only after a completed Workflow", async () => {
    const ledger = { complete: vi.fn(async () => undefined), release: vi.fn(async () => undefined), claim: vi.fn() };
    const activities = createAgentEventLedgerLifecycleActivities(ledger);

    await activities.settleInboundEvent!({ claim, status: "completed" });

    expect(ledger.complete).toHaveBeenCalledWith(claim);
    expect(ledger.release).not.toHaveBeenCalled();
  });

  it.each(["failed", "cancelled"] as const)("releases a %s Workflow for reclaim", async status => {
    const ledger = { complete: vi.fn(async () => undefined), release: vi.fn(async () => undefined), claim: vi.fn() };
    const activities = createAgentEventLedgerLifecycleActivities(ledger);

    await activities.settleInboundEvent!({ claim, status, error: "transient failure" });

    expect(ledger.complete).not.toHaveBeenCalled();
    expect(ledger.release).toHaveBeenCalledWith(claim, "transient failure");
  });
});
