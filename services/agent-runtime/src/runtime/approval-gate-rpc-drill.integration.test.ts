import { createHash } from "node:crypto";

import { afterAll, describe, expect, it } from "vitest";
import { z } from "zod";

import { CapabilityRegistry } from "../capabilities/registry.js";
import type { AgentApprovalBinding } from "../capabilities/agent-capability-rpc.js";
import { canonicalMcpJSON } from "../mcp/canonical-json.js";
import {
  createMcpWriteApprovalConsumePort,
  createMcpWriteApprovalGrantResolver,
  McpWriteApprovalGate
} from "../mcp/mcp-write-approval-gate.js";
import { executionContextSchema, type ExecutionContext } from "./execution-context.js";
import { createAgentCapabilityRPC, type ShadowRuntimeConfig } from "./shadow-runtime.js";

const enabled = process.env.DIPOLE_AGENT_APPROVAL_GATE_DRILL === "true";
const integration = describe.skipIf(!enabled);

integration("Agent Approval gate mTLS drill", () => {
  const rpc = createAgentCapabilityRPC(config());

  afterAll(() => rpc.close());

  it("allows one exact write, denies rejected and replayed grants, and preserves consumption after failure", async () => {
    let effectCount = 0;
    const context = executionContext("TASK-APPROVAL-ALLOW", "RUN-APPROVAL-ALLOW");
    const gate = gateFor(async () => {
      effectCount += 1;
      return { messageId: "MSG-APPROVAL-ALLOW" };
    });
    await approve("APR-APPROVAL-ALLOW", context);

    await expect(gate.execute("message.system.send", input(), context))
      .resolves.toEqual({ messageId: "MSG-APPROVAL-ALLOW" });
    expect(effectCount).toBe(1);
    await expect(gate.execute("message.system.send", input(), context)).rejects.toThrow(/Approval is unavailable/i);
    expect(effectCount).toBe(1);

    const deniedContext = executionContext("TASK-APPROVAL-DENY", "RUN-APPROVAL-DENY");
    await reject("APR-APPROVAL-DENY", deniedContext);
    await expect(gate.execute("message.system.send", input(), deniedContext)).rejects.toThrow(/Approval is unavailable/i);
    expect(effectCount).toBe(1);

    const failureContext = executionContext("TASK-APPROVAL-FAIL", "RUN-APPROVAL-FAIL");
    let failingEffectCount = 0;
    const failingGate = gateFor(async () => {
      failingEffectCount += 1;
      throw new Error("isolated write failure");
    });
    await approve("APR-APPROVAL-FAIL", failureContext);
    await expect(failingGate.execute("message.system.send", input(), failureContext)).rejects.toThrow("isolated write failure");
    await expect(failingGate.execute("message.system.send", input(), failureContext)).rejects.toThrow(/Approval is unavailable/i);
    expect(failingEffectCount).toBe(1);
  });

  function gateFor(execute: () => Promise<unknown>): McpWriteApprovalGate {
    const registry = new CapabilityRegistry();
    registry.register({
      descriptor: {
        id: "message.system.send", risk: "write", requiredPermission: "message.write", approvalRequired: true
      },
      inputSchema: z.object({ conversationId: z.literal("group:G1"), content: z.literal("isolated approval drill") }).strict(),
      resolveResource: value => ({ resourceType: "conversation", resourceId: value.conversationId, action: "write" }),
      execute
    });
    return new McpWriteApprovalGate(
      registry,
      createMcpWriteApprovalConsumePort(rpc.client),
      createMcpWriteApprovalGrantResolver(rpc.client)
    );
  }

  async function approve(approvalId: string, context: ExecutionContext): Promise<void> {
    const binding = approvalBinding(approvalId);
    await rpc.client.requestApproval(context.taskId, context.runId, binding, context);
    await rpc.client.resolveApproval(context.taskId, context.runId, approvalId, "approved", "U100", context);
  }

  async function reject(approvalId: string, context: ExecutionContext): Promise<void> {
    const binding = approvalBinding(approvalId);
    await rpc.client.requestApproval(context.taskId, context.runId, binding, context);
    await rpc.client.resolveApproval(context.taskId, context.runId, approvalId, "denied", "U100", context);
  }
});

function config(): ShadowRuntimeConfig {
  return {
    runtimeMode: "active",
    candidateVersion: "approval-gate-drill-v1",
    capabilityRpc: {
      enabled: true,
      target: requiredEnv("DIPOLE_TEST_AGENT_RPC_TARGET"),
      secret: requiredEnv("DIPOLE_TEST_AGENT_RPC_SECRET"),
      timeoutMs: 2_000,
      tls: {
        enabled: true,
        caFile: requiredEnv("DIPOLE_TEST_AGENT_RPC_CA_FILE"),
        certFile: requiredEnv("DIPOLE_TEST_AGENT_RPC_CERT_FILE"),
        keyFile: requiredEnv("DIPOLE_TEST_AGENT_RPC_KEY_FILE"),
        serverName: requiredEnv("DIPOLE_TEST_AGENT_RPC_SERVER_NAME")
      }
    }
  } as ShadowRuntimeConfig;
}

function executionContext(taskId: string, runId: string): ExecutionContext {
  return executionContextSchema.parse({
    tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI-DRILL", taskId, runId, mode: "active",
    permissions: ["message.write"], resourceScopes: [{ resourceType: "conversation", resourceId: "group:G1", actions: ["write"] }],
    approvedCapabilities: ["message.system.send"], requestId: `REQ-${taskId}`, traceId: `TRACE-${taskId}`
  });
}

function approvalBinding(approvalId: string): AgentApprovalBinding {
  const args = input();
  return {
    approvalId, capabilityId: "message.system.send",
    resourceScope: { resourceType: "conversation", resourceId: "group:G1", actions: ["write"] },
    scopeSha256: sha256("dipole.agent.scope.v1\nconversation\ngroup:G1\nwrite"),
    argumentsSha256: sha256(canonicalMcpJSON(args)),
    nonceSha256: sha256(`nonce:${approvalId}`),
    expiresAtUnixMs: Date.now() + 60_000
  };
}

function input() {
  return { conversationId: "group:G1", content: "isolated approval drill" };
}

function sha256(value: string): string {
  return createHash("sha256").update(value, "utf8").digest("hex");
}

function requiredEnv(name: string): string {
  const value = process.env[name]?.trim();
  if (!value) throw new Error(`${name} is required for the Approval gate drill`);
  return value;
}
