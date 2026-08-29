import { describe, expect, it } from "vitest";

import { InMemoryEventLedger } from "./event-ledger.js";

describe("InMemoryEventLedger", () => {
  it("grants one concurrent claim and exposes completed duplicates", async () => {
    const ledger = new InMemoryEventLedger();
    const claims = await Promise.all(Array.from({ length: 16 }, () => ledger.claim("E1", "TASK-1")));
    const granted = claims.filter((claim) => claim !== undefined);
    expect(granted).toHaveLength(1);
    await ledger.complete(granted[0]!);
    await expect(ledger.claim("E1", "TASK-1")).resolves.toBeUndefined();
  });

  it("allows retry after an exact claim is released", async () => {
    const ledger = new InMemoryEventLedger();
    const claim = await ledger.claim("E1", "TASK-1");
    expect(claim).toBeDefined();
    await ledger.release(claim!);
    await expect(ledger.claim("E1", "TASK-1")).resolves.toBeDefined();
  });

  it("rejects an event ID rebound to a different Task ID", async () => {
    const ledger = new InMemoryEventLedger();
    await ledger.claim("E1", "TASK-1");
    await expect(ledger.claim("E1", "TASK-2")).rejects.toThrow(/binding conflict/);
  });
});
