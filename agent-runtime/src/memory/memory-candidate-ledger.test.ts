import { describe, expect, it, vi } from "vitest";

import { ObservationWorker } from "./observation-worker.js";
import { MySQLMemoryCandidateLedger, type MemoryCandidateLedgerQueryExecutor } from "./memory-candidate-ledger.js";

const candidate = new ObservationWorker().observe({
  tenantId: "dipole", principalId: "U100", agentId: "UAI", resourceType: "conversation", resourceId: "group:G1",
  eventId: "EV-1", messageId: "M-1", messageSequence: "42", senderId: "U200",
  occurredAt: "2026-08-29T00:00:00.000Z", content: "决定：周五前完成 API v2。",
})[0]!;
const candidateWithPrivateBody = { ...candidate, content: `${candidate.content} ${"private-body ".repeat(80)}` };

function executor(): MemoryCandidateLedgerQueryExecutor & { calls: unknown[][] } {
  const calls: unknown[][] = [];
  return {
    calls,
    execute: vi.fn(async (sql: string, values?: unknown[]): Promise<[unknown, unknown]> => {
      calls.push([sql, values]);
      if (sql.startsWith("INSERT")) return [{ affectedRows: 1 }, []];
      return [[], []];
    }),
  };
}

describe("MySQLMemoryCandidateLedger", () => {
  it("stores only compact summary, evidence, policy and hash", async () => {
    const db = executor();
    const result = await new MySQLMemoryCandidateLedger(db).append(candidateWithPrivateBody, ["M-1"], "observation-v1");

    expect(result).toEqual({ outcome: "inserted" });
    const values = db.calls[0]?.[1] as unknown[];
    expect(values).toEqual(expect.arrayContaining([candidate.compactContent, '["M-1"]', "observation-v1"]));
    expect(values).not.toContain(candidateWithPrivateBody.content);
    expect(values.at(-2)).toMatch(/^[a-f0-9]{64}$/);
  });

  it("returns duplicate for an exact replay and rejects a hash conflict", async () => {
    const db = executor();
    db.execute = vi.fn()
      .mockResolvedValueOnce([{ affectedRows: 0 }, []])
      .mockResolvedValueOnce([[{ candidate_sha256: expectHash(candidate, ["M-1"], "observation-v1") }], []]);
    await expect(new MySQLMemoryCandidateLedger(db).append(candidate, ["M-1"], "observation-v1")).resolves.toEqual({ outcome: "duplicate" });

    const conflict = executor();
    conflict.execute = vi.fn()
      .mockResolvedValueOnce([{ affectedRows: 0 }, []])
      .mockResolvedValueOnce([[{ candidate_sha256: "f".repeat(64) }], []]);
    await expect(new MySQLMemoryCandidateLedger(conflict).append(candidate, ["M-1"], "observation-v1")).rejects.toThrow(/conflict/i);
  });

  it("fails closed for empty, duplicate or credential-shaped evidence", async () => {
    const db = executor();
    const ledger = new MySQLMemoryCandidateLedger(db);
    await expect(ledger.append(candidate, [], "observation-v1")).rejects.toThrow(/evidence/i);
    await expect(ledger.append(candidate, ["M-1", "M-1"], "observation-v1")).rejects.toThrow(/evidence/i);
    await expect(ledger.append({ ...candidate, compactContent: "token=secret" }, ["M-1"], "observation-v1")).rejects.toThrow(/summary/i);
  });
});

function expectHash(value: typeof candidate, evidenceIds: string[], policyVersion: string): string {
  return new MySQLMemoryCandidateLedger(executor()).candidateHash(value, evidenceIds, policyVersion);
}
