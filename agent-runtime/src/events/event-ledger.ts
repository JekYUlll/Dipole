import { randomUUID } from "node:crypto";

export interface EventClaim {
  readonly eventId: string;
  readonly taskId: string;
  readonly token: string;
}

export interface EventLedger {
  claim(eventId: string, taskId: string, eventType?: string): Promise<EventClaim | undefined>;
  complete(claim: EventClaim): Promise<void>;
  release(claim: EventClaim, error?: unknown): Promise<void>;
}

type LedgerEntry = { taskId: string; token: string; status: "claimed" | "completed" };

export class InMemoryEventLedger implements EventLedger {
  readonly #entries = new Map<string, LedgerEntry>();

  async claim(eventId: string, taskId: string, _eventType?: string): Promise<EventClaim | undefined> {
    eventId = eventId.trim();
    taskId = taskId.trim();
    if (!eventId || !taskId) {
      throw new Error("event ledger requires event and Task IDs");
    }
    const existing = this.#entries.get(eventId);
    if (existing !== undefined) {
      if (existing.taskId !== taskId) {
        throw new Error(`event ledger binding conflict for ${eventId}`);
      }
      return undefined;
    }
    const claim = { eventId, taskId, token: randomUUID() };
    this.#entries.set(eventId, { taskId, token: claim.token, status: "claimed" });
    return claim;
  }

  async complete(claim: EventClaim): Promise<void> {
    const entry = this.requireExactClaim(claim);
    entry.status = "completed";
  }

  async release(claim: EventClaim, _error?: unknown): Promise<void> {
    this.requireExactClaim(claim);
    this.#entries.delete(claim.eventId);
  }

  private requireExactClaim(claim: EventClaim): LedgerEntry {
    const entry = this.#entries.get(claim.eventId);
    if (entry === undefined || entry.taskId !== claim.taskId || entry.token !== claim.token || entry.status !== "claimed") {
      throw new Error(`event ledger claim is stale for ${claim.eventId}`);
    }
    return entry;
  }
}
