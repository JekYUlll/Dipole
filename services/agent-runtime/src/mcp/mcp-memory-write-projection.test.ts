import { createHash } from "node:crypto";
import { describe, expect, it, vi } from "vitest";

import { canonicalMcpJSON } from "./canonical-json.js";
import { createInteractiveMemoryExecutor } from "./mcp-memory-write-projection.js";

const context = {
  tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId: "TASK-1", runId: "RUN-1", mode: "active" as const,
  permissions: ["memory.write"], resourceScopes: [{ resourceType: "conversation", resourceId: "direct:U100:UAI", actions: ["write"] }],
  approvedCapabilities: ["memory.save"] as "memory.save"[], eventId: "E1"
};

describe("interactive Memory write", () => {
  it("consumes the exact approval, executes one audited command, and preserves the bound scope", async () => {
    const input = {
      conversationId: "direct:U100:UAI", memoryType: "semantic" as const, content: "Prefer concise Chinese replies"
    };
    const resourceScope = { resourceType: "conversation", resourceId: input.conversationId, actions: ["write"] };
    const grant = {
      approvalId: "APR-1", capabilityId: "memory.save", resourceScope,
      scopeSha256: sha256(["dipole.agent.scope.v1", resourceScope.resourceType, resourceScope.resourceId, ...resourceScope.actions].join("\n")),
      argumentsSha256: sha256(canonicalMcpJSON(input)), nonceSha256: "c".repeat(64), expiresAtUnixMs: Date.now() + 60_000
    };
    const beginMcpToolCommand = vi.fn()
      .mockRejectedValueOnce({ code: 7 })
      .mockResolvedValue({ invocationId: "tool:memory", status: "running" as const });
    const consumeApproval = vi.fn(async () => undefined);
    const executeMemoryCommand = vi.fn(async () => ({
      memoryId: "MEM-1", memoryType: "semantic" as const, content: "Prefer concise Chinese replies", priority: 500,
      provenance: { sourceType: "agent_task", sourceId: "TASK-1", sequence: "RUN-1" }
    }));
    const finishToolInvocation = vi.fn(async () => undefined);
    const executor = createInteractiveMemoryExecutor({
      beginMcpToolCommand, consumeApproval, executeMemoryCommand, finishToolInvocation,
      resolveApprovalGrant: vi.fn(async () => grant)
    } as never);

    await expect(executor.execute(input, context, "APR-1")).resolves.toMatchObject({ memoryId: "MEM-1" });

    expect(consumeApproval).toHaveBeenCalledOnce();
    expect(executeMemoryCommand).toHaveBeenCalledOnce();
    expect(executeMemoryCommand).toHaveBeenCalledWith(expect.objectContaining({
      taskId: "TASK-1", runId: "RUN-1", memoryType: "semantic", conversationKey: "direct:U100:UAI"
    }));
    expect(finishToolInvocation).toHaveBeenCalledOnce();
  });

  it("rejects a Memory target outside the approval scope", async () => {
    const executor = createInteractiveMemoryExecutor({} as never);
    await expect(executor.execute({
      conversationId: "group:G1", memoryType: "episodic", content: "A decision"
    }, context, "APR-1")).rejects.toThrow(/scope|resource/i);
  });
});

function sha256(value: string): string {
  return createHash("sha256").update(value, "utf8").digest("hex");
}
