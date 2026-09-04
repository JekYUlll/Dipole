// Token lifecycle state machine for Runtime-owned OAuth exchange results.
//
// Purpose: track a single handoff's token bundle through
// `pending_exchange → active → refreshed → { revoked | expired }` transitions,
// with a retention rule that redacts secret material once a record reaches a
// terminal state (`revoked` or `expired`).
//
// Default assembly: unmounted. The processor constructs a store only when a
// caller explicitly wires it; there is no bootstrap-side singleton, no Core
// SQLC table, no migration. Production adopters will replace this seam with a
// Core-owned durable store; the in-memory implementation is only meant for
// offline composition tests.
//
// Boundary: this file owns state transitions only. It never opens envelopes,
// never talks to a provider, never persists to disk. It also never emits
// plaintext access tokens after a terminal transition — `get` returns the
// record with `accessToken` and `refreshToken` fields nulled.

export type TokenLifecycleState =
  | "pending_exchange"
  | "active"
  | "refreshed"
  | "revoked"
  | "expired";

export interface TokenLifecycleTimestamps {
  readonly createdAt: Date;
  readonly updatedAt: Date;
  readonly terminalAt?: Date;
}

export interface TokenLifecycleRecord {
  readonly handoffId: string;
  readonly state: TokenLifecycleState;
  readonly accessToken: string | null;
  readonly refreshToken: string | null;
  readonly tokenType: string | null;
  readonly expiresAt: Date | null;
  readonly scope: string | null;
  readonly revocationReason?: string;
  readonly refreshCount: number;
  readonly timestamps: TokenLifecycleTimestamps;
}

export interface TokenLifecycleBundle {
  readonly accessToken: string;
  readonly refreshToken?: string;
  readonly tokenType: string;
  readonly expiresAt: Date;
  readonly scope?: string;
}

export class TokenLifecycleInvalidTransitionError extends Error {}

const handoffIDPattern = /^[A-Za-z0-9_-]{16,64}$/;

/** In-memory token lifecycle store. Not for production; see file header. */
export class TokenLifecycleStore {
  private readonly records: Map<string, TokenLifecycleRecord> = new Map();
  private readonly now: () => Date;

  constructor(now: () => Date = () => new Date()) {
    this.now = now;
  }

  /** Marks `pending_exchange` so a caller can observe an in-flight handoff before the provider is called. */
  markPending(handoffId: string): TokenLifecycleRecord {
    this.assertHandoffId(handoffId);
    const existing = this.records.get(handoffId);
    if (existing !== undefined) return existing;
    const now = this.now();
    const record: TokenLifecycleRecord = Object.freeze({
      handoffId, state: "pending_exchange", accessToken: null, refreshToken: null, tokenType: null,
      expiresAt: null, scope: null, refreshCount: 0, timestamps: Object.freeze({ createdAt: now, updatedAt: now })
    });
    this.records.set(handoffId, record);
    return record;
  }

  /** Records the initial exchange result. Legal only from `pending_exchange` or when no record exists. */
  upsertExchange(handoffId: string, bundle: TokenLifecycleBundle): TokenLifecycleRecord {
    this.assertHandoffId(handoffId);
    const existing = this.records.get(handoffId);
    if (existing !== undefined && existing.state !== "pending_exchange") {
      throw new TokenLifecycleInvalidTransitionError(`cannot upsert exchange from state ${existing.state}`);
    }
    const now = this.now();
    const record: TokenLifecycleRecord = Object.freeze({
      handoffId, state: "active",
      accessToken: bundle.accessToken,
      refreshToken: bundle.refreshToken ?? null,
      tokenType: bundle.tokenType,
      expiresAt: bundle.expiresAt,
      scope: bundle.scope ?? null,
      refreshCount: 0,
      timestamps: Object.freeze({ createdAt: existing?.timestamps.createdAt ?? now, updatedAt: now })
    });
    this.records.set(handoffId, record);
    return record;
  }

  /** Refreshes an active or already-refreshed record with a new bundle. */
  refresh(handoffId: string, bundle: TokenLifecycleBundle): TokenLifecycleRecord {
    this.assertHandoffId(handoffId);
    const existing = this.records.get(handoffId);
    if (existing === undefined || (existing.state !== "active" && existing.state !== "refreshed")) {
      throw new TokenLifecycleInvalidTransitionError(`cannot refresh from state ${existing?.state ?? "missing"}`);
    }
    const now = this.now();
    const record: TokenLifecycleRecord = Object.freeze({
      handoffId, state: "refreshed",
      accessToken: bundle.accessToken,
      refreshToken: bundle.refreshToken ?? existing.refreshToken,
      tokenType: bundle.tokenType,
      expiresAt: bundle.expiresAt,
      scope: bundle.scope ?? existing.scope,
      refreshCount: existing.refreshCount + 1,
      timestamps: Object.freeze({ createdAt: existing.timestamps.createdAt, updatedAt: now })
    });
    this.records.set(handoffId, record);
    return record;
  }

  /** Records a permanent revocation. Legal from any non-terminal state or when no record exists. */
  revoke(handoffId: string, reason: string): TokenLifecycleRecord {
    this.assertHandoffId(handoffId);
    if (typeof reason !== "string" || reason.length === 0 || reason.length > 512) {
      throw new TokenLifecycleInvalidTransitionError("revocation reason is invalid");
    }
    const existing = this.records.get(handoffId);
    if (existing !== undefined && (existing.state === "revoked" || existing.state === "expired")) {
      throw new TokenLifecycleInvalidTransitionError(`cannot revoke from state ${existing.state}`);
    }
    const now = this.now();
    const record: TokenLifecycleRecord = Object.freeze({
      handoffId, state: "revoked",
      accessToken: null, refreshToken: null,
      tokenType: existing?.tokenType ?? null,
      expiresAt: existing?.expiresAt ?? null,
      scope: existing?.scope ?? null,
      revocationReason: reason,
      refreshCount: existing?.refreshCount ?? 0,
      timestamps: Object.freeze({
        createdAt: existing?.timestamps.createdAt ?? now,
        updatedAt: now,
        terminalAt: now
      })
    });
    this.records.set(handoffId, record);
    return record;
  }

  /** Marks a record expired when its `expiresAt` has passed. Legal only from `active` or `refreshed`. */
  expire(handoffId: string, now: Date = this.now()): TokenLifecycleRecord {
    this.assertHandoffId(handoffId);
    const existing = this.records.get(handoffId);
    if (existing === undefined || (existing.state !== "active" && existing.state !== "refreshed")) {
      throw new TokenLifecycleInvalidTransitionError(`cannot expire from state ${existing?.state ?? "missing"}`);
    }
    if (existing.expiresAt === null || now.getTime() < existing.expiresAt.getTime()) {
      throw new TokenLifecycleInvalidTransitionError("token has not yet expired");
    }
    const record: TokenLifecycleRecord = Object.freeze({
      handoffId, state: "expired",
      accessToken: null, refreshToken: null,
      tokenType: existing.tokenType,
      expiresAt: existing.expiresAt,
      scope: existing.scope,
      refreshCount: existing.refreshCount,
      timestamps: Object.freeze({ createdAt: existing.timestamps.createdAt, updatedAt: now, terminalAt: now })
    });
    this.records.set(handoffId, record);
    return record;
  }

  get(handoffId: string): TokenLifecycleRecord | undefined {
    return this.records.get(handoffId);
  }

  private assertHandoffId(handoffId: string): void {
    if (!handoffIDPattern.test(handoffId)) {
      throw new TokenLifecycleInvalidTransitionError("handoff id is invalid");
    }
  }
}

/** True when a record is in a state that must not trigger another provider exchange. */
export function isTerminalOrActive(record: TokenLifecycleRecord): boolean {
  return record.state === "active" || record.state === "refreshed" || record.state === "revoked" || record.state === "expired";
}
