import * as grpc from "@grpc/grpc-js";
import { readFile } from "node:fs/promises";
import { describe, expect, it } from "vitest";
import { TestWorkflowEnvironment } from "@temporalio/testing";
import { Worker } from "@temporalio/worker";

import { AgentCapabilityServiceClient, type IAgentCapabilityServiceClient } from "../generated/dipole/agent/v1/agent.grpc-client.js";
import { AgentCapabilityRPCClient } from "../capabilities/agent-capability-rpc.js";
import { createAgentMemoryPromotionCommitActivities } from "./agent-memory-promotion-commit-activity.js";
import { foundationAgentTaskActivities, type AgentMemoryPromotionActivities, type AgentTaskWorkerActivities } from "./agent-task-activities.js";

const enabled = process.env.DIPOLE_AGENT_TEMPORAL_MYSQL_MTLS_INTEGRATION === "true";

type Fixture = {
  Target: string; Secret: string; CAFile: string; CertFile: string; KeyFile: string; ServerName: string;
  TenantID: string; PrincipalUserID: string; AgentID: string; TaskID: string; RunID: string;
  CandidateID: string; CandidateSHA256: string; ReviewID: string; PolicyVersion: string;
};

describe.skipIf(!enabled)("Temporal Agent Memory promotion through Core mTLS and MySQL", () => {
  it("retries an Activity after Core committed and reuses the same persistent Memory", async () => {
    const fixture = JSON.parse(await readFile(requiredEnv("DIPOLE_AGENT_TEMPORAL_MYSQL_MTLS_FIXTURE"), "utf8")) as Fixture;
    const ca = await readFile(fixture.CAFile);
    const certificate = await readFile(fixture.CertFile);
    const key = await readFile(fixture.KeyFile);
    const transport = new AgentCapabilityServiceClient(fixture.Target, grpc.credentials.createSsl(ca, key, certificate), {
      "grpc.ssl_target_name_override": fixture.ServerName,
      "grpc.default_authority": fixture.ServerName
    });
    const client = new AgentCapabilityRPCClient(
      transport as unknown as IAgentCapabilityServiceClient, fixture.Secret, 2_000, "active", "receipt-temporal-mysql-mtls-v1"
    );
    const temporal = await TestWorkflowEnvironment.createLocal();
    let calls = 0;
    const committed = createAgentMemoryPromotionCommitActivities(client);
    const activities: AgentTaskWorkerActivities & Pick<AgentMemoryPromotionActivities, "commitPreparedAgentMemoryPromotion"> = {
      ...foundationAgentTaskActivities,
      async admitAgentTask() { return { taskId: fixture.TaskID, runId: fixture.RunID, runStatus: "running" }; },
      async executeAgentTaskStep() { return { kind: "complete", output: { outcome: "committed-through-mtls" } }; },
      async commitPreparedAgentMemoryPromotion(input) {
        const result = await committed.commitPreparedAgentMemoryPromotion(input);
        calls += 1;
        if (calls === 1) throw new Error("simulated Worker failure after durable Core commit");
        return result;
      }
    };
    const taskQueue = `dipole-agent-receipt-mtls-mysql-${Date.now()}`;
    const worker = await Worker.create({
      connection: temporal.nativeConnection,
      ...(temporal.namespace === undefined ? {} : { namespace: temporal.namespace }),
      taskQueue,
      workflowsPath: new URL("./agent-task-workflow.ts", import.meta.url).pathname,
      activities
    });
    const expiresAt = new Date(Date.now() + 10 * 60 * 1_000).toISOString();
    try {
      const result = await worker.runUntil(() => temporal.client.workflow.execute("agentTaskWorkflow", {
        taskQueue,
        workflowId: `dipole-agent-task/receipt-mtls-mysql-${Date.now()}`,
        args: [{
          taskId: fixture.TaskID, goal: "commit reviewed Memory through Core mTLS",
          memoryPromotion: {
            tenantId: fixture.TenantID, principalUserId: fixture.PrincipalUserID, agentId: fixture.AgentID,
            taskId: fixture.TaskID, runId: fixture.RunID, candidateId: fixture.CandidateID,
            candidateSha256: fixture.CandidateSHA256, reviewId: fixture.ReviewID, policyVersion: fixture.PolicyVersion,
            candidateMemoryType: "observational", targetMemoryType: "semantic", expiresAt, commit: true
          },
          admission: { tenantId: fixture.TenantID, principalUserId: fixture.PrincipalUserID, agentId: fixture.AgentID, triggerType: "manual", triggerRef: fixture.CandidateID }
        }]
      })) as { status: string; output?: { promotionReceipt?: { receiptSha256: string }; promotionCommit?: { memoryId: string; receiptSha256: string } } };
      expect(result.status).toBe("completed");
      expect(calls).toBe(2);
      expect(result.output?.promotionCommit?.receiptSha256).toBe(result.output?.promotionReceipt?.receiptSha256);
      expect(result.output?.promotionCommit?.memoryId).toMatch(/^MEM-/);
    } finally {
      transport.close();
      await temporal.teardown();
    }
  }, 120_000);
});

function requiredEnv(name: string): string {
  const value = process.env[name]?.trim();
  if (!value) throw new Error(`${name} is required for Temporal/MySQL mTLS integration`);
  return value;
}
