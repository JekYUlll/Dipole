import * as grpc from "@grpc/grpc-js";
import { readFile } from "node:fs/promises";
import { describe, expect, it } from "vitest";

import { AgentCapabilityServiceClient, type IAgentCapabilityServiceClient } from "../generated/dipole/agent/v1/agent.grpc-client.js";
import { createAgentMemoryPromotionReceipt } from "../memory/agent-memory-promotion-receipt.js";
import { AgentCapabilityRPCClient } from "./agent-capability-rpc.js";

const enabled = process.env.DIPOLE_AGENT_MEMORY_PROMOTION_RPC_DRILL === "true";

describe.skipIf(!enabled)("Agent Memory promotion receipt mTLS RPC drill", () => {
  it("commits an exact receipt through the Go mTLS fixture", async () => {
    const target = requiredEnv("DIPOLE_TEST_AGENT_PROMOTION_RPC_TARGET");
    const ca = await readFile(requiredEnv("DIPOLE_TEST_AGENT_PROMOTION_RPC_CA_FILE"));
    const certificate = await readFile(requiredEnv("DIPOLE_TEST_AGENT_PROMOTION_RPC_CERT_FILE"));
    const key = await readFile(requiredEnv("DIPOLE_TEST_AGENT_PROMOTION_RPC_KEY_FILE"));
    const transport = new AgentCapabilityServiceClient(target, grpc.credentials.createSsl(ca, key, certificate), {
      "grpc.ssl_target_name_override": requiredEnv("DIPOLE_TEST_AGENT_PROMOTION_RPC_SERVER_NAME"),
      "grpc.default_authority": requiredEnv("DIPOLE_TEST_AGENT_PROMOTION_RPC_SERVER_NAME")
    });
    const client = new AgentCapabilityRPCClient(
      transport as unknown as IAgentCapabilityServiceClient,
      requiredEnv("DIPOLE_TEST_AGENT_PROMOTION_RPC_SECRET"),
      2_000,
      "active",
      "agent-runtime@promotion-rpc-drill"
    );
    const createdAt = new Date();
    const receipt = createAgentMemoryPromotionReceipt({
      tenantId: "dipole", principalUserId: "U100", agentId: "UAI", taskId: "TASK-MTLS-1", runId: "RUN-MTLS-1",
      candidateId: "CAND-MTLS-1", candidateSha256: "a".repeat(64), reviewId: "REV-MTLS-1", policyVersion: "memory-v1",
      candidateMemoryType: "observational", targetMemoryType: "semantic", expiresAt: new Date(createdAt.getTime() + 10 * 60 * 1_000).toISOString()
    }, createdAt);

    try {
      await expect(client.commitMemoryPromotionReceipt(receipt, { requestId: "REQ-MTLS-1", traceId: "TRACE-MTLS-1" }))
        .resolves.toEqual({
          memoryId: "MEM-COMMIT-CAND-MTLS-1", memoryType: "semantic", status: "active", receiptSha256: receipt.receiptSha256,
          provenance: { sourceType: "memory_candidate", sourceId: "CAND-MTLS-1", sequence: "REV-MTLS-1" }
        });
    } finally {
      transport.close();
    }
  });
});

function requiredEnv(name: string): string {
  const value = process.env[name]?.trim();
  if (!value) throw new Error(`${name} is required for the Agent Memory promotion RPC drill`);
  return value;
}
