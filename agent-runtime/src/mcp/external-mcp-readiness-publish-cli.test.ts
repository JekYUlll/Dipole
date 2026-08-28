import { describe, expect, it, vi } from "vitest";

import { runExternalMcpReadinessPublishCLI } from "./external-mcp-readiness-publish-cli.js";

describe("external MCP readiness publication CLI", () => {
  it("runs one explicit publication and emits only its low-sensitive receipt", async () => {
    const output: string[] = [];
    const errors: string[] = [];
    const close = vi.fn();
    const publish = vi.fn(async () => ({
      evidenceId: "c".repeat(64), profileBindingSha256: "b".repeat(64), runtimeBindingSha256: "a".repeat(64),
      contentSha256: "d".repeat(64), collectedAt: "2026-08-28T14:00:03.000Z",
      expiresAt: "2026-08-28T14:30:03.000Z", created: true
    }));

    await expect(runExternalMcpReadinessPublishCLI([
      "--tenant=TENANT-A", "--profile=PROFILE-A", "--valid-for-seconds=1800",
      "--request-id=REQ-1", "--trace-id=TRACE-1"
    ], writer(output), writer(errors), { openPublication: async () => ({ publish, close }) })).resolves.toBe(0);

    expect(publish).toHaveBeenCalledWith({
      tenantId: "TENANT-A", profileId: "PROFILE-A", validForMs: 1_800_000,
      requestId: "REQ-1", traceId: "TRACE-1"
    });
    expect(close).toHaveBeenCalledOnce();
    expect(errors).toEqual([]);
    expect(JSON.parse(output.join(""))).toEqual(expect.objectContaining({ evidenceId: "c".repeat(64), created: true }));
    expect(output.join("")).not.toContain("PROFILE-A");
  });

  it("rejects missing or extra arguments before opening dependencies", async () => {
    const openPublication = vi.fn();
    const errors: string[] = [];
    await expect(runExternalMcpReadinessPublishCLI([], writer([]), writer(errors), { openPublication })).resolves.toBe(1);
    await expect(runExternalMcpReadinessPublishCLI([
      "--tenant=T", "--profile=P", "--valid-for-seconds=60", "--request-id=R", "--trace-id=X", "--extra=true"
    ], writer([]), writer(errors), { openPublication })).resolves.toBe(1);
    await expect(runExternalMcpReadinessPublishCLI([
      "--tenant= TENANT-A", "--profile=PROFILE-A", "--valid-for-seconds=1800",
      "--request-id=REQ-1", "--trace-id=TRACE-1"
    ], writer([]), writer(errors), { openPublication })).resolves.toBe(1);
    expect(openPublication).not.toHaveBeenCalled();
  });

  it("fails closed and closes dependencies after a publication error", async () => {
    const close = vi.fn();
    const errors: string[] = [];
    const code = await runExternalMcpReadinessPublishCLI([
      "--tenant=TENANT-A", "--profile=PROFILE-A", "--valid-for-seconds=1800",
      "--request-id=REQ-1", "--trace-id=TRACE-1"
    ], writer([]), writer(errors), {
      openPublication: async () => ({ publish: async () => { throw new Error("SECRET-VALUE"); }, close })
    });
    expect(code).toBe(1);
    expect(close).toHaveBeenCalledOnce();
    expect(errors.join("")).toBe("external MCP readiness publication failed closed\n");
  });
});

function writer(values: string[]) {
  return { write: (value: string) => { values.push(String(value)); return true; } };
}
