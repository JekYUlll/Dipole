import { describe, expect, it, vi } from "vitest";
import type { Pool } from "mysql2/promise";

import { MySQLShadowAuditSink } from "./mysql-shadow-audit-sink.js";

describe("MySQLShadowAuditSink", () => {
  it("persists bigint capability output as a decimal JSON string", async () => {
    const execute = vi.fn(async () => [{ affectedRows: 1 }, undefined]);
    const sink = new MySQLShadowAuditSink({ execute } as unknown as Pool);

    await expect(sink.completeStep("TASK-1", 1, "TOKEN-1", {
      conversation: { lastMessageSeq: 42n },
      events: [{ eventSeq: 9007199254740993n }]
    })).resolves.toBeUndefined();

    expect(execute).toHaveBeenCalledWith(expect.any(String), [
      '{"conversation":{"lastMessageSeq":"42"},"events":[{"eventSeq":"9007199254740993"}]}',
      "TASK-1", 1, "TOKEN-1"
    ]);
  });
});
