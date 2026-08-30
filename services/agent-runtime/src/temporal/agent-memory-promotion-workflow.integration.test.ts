import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { TestWorkflowEnvironment } from "@temporalio/testing";
import { Worker } from "@temporalio/worker";

import {
  foundationAgentTaskActivities,
  type AgentMemoryPromotionActivities,
  type AgentTaskWorkerActivities
} from "./agent-task-activities.js";

const enabled = process.env.DIPOLE_AGENT_TEMPORAL_INTEGRATION === "true";

describe.skipIf(!enabled)("Temporal Agent Memory promotion intent", () => {
  let env: TestWorkflowEnvironment;

  beforeAll(async () => {
    env = await TestWorkflowEnvironment.createLocal();
  }, 120_000);

  afterAll(async () => {
    await env?.teardown();
  });

  it("keeps an exact promotion receipt in the durable Task result", async () => {
    const taskQueue = `dipole-agent-memory-promotion-${Date.now()}`;
    const expiresAt = new Date(Date.now() + 10 * 60 * 1000).toISOString();
    const activities: AgentTaskWorkerActivities = {
      ...foundationAgentTaskActivities,
      async admitAgentTask(input) {
        return { taskId: input.taskId, runId: "RUN-1", runStatus: "running" };
      },
      async executeAgentTaskStep() {
        return { kind: "complete", output: { outcome: "prepared" } };
      }
    };
    const worker = await Worker.create({
      connection: env.nativeConnection,
      ...(env.namespace === undefined ? {} : { namespace: env.namespace }),
      taskQueue,
      workflowsPath: new URL("./agent-task-workflow.ts", import.meta.url).pathname,
      activities
    });
    const result = await worker.runUntil(() => env.client.workflow.execute("agentTaskWorkflow", {
      taskQueue,
      workflowId: `dipole-agent-task/memory-promotion-${Date.now()}`,
      args: [{
        taskId: "TASK-MEM-PROMOTE", goal: "prepare reviewed memory promotion",
        memoryPromotion: {
          tenantId: "dipole", principalUserId: "U100", agentId: "UAI", taskId: "TASK-MEM-PROMOTE", runId: "RUN-1",
          candidateId: "CAND-1", candidateSha256: "a".repeat(64), reviewId: "REV-1", policyVersion: "memory-v1",
          candidateMemoryType: "observational", targetMemoryType: "semantic", expiresAt
        },
        admission: {
          tenantId: "dipole", principalUserId: "U100", agentId: "UAI",
          triggerType: "manual", triggerRef: "CAND-1", eventId: "EVENT-1"
        }
      }]
    })) as { status: string; output?: { result?: unknown; promotionReceipt?: { status: string; candidateId: string; reviewId: string } } };
    expect(result.status).toBe("completed");
    expect(result.output).toMatchObject({
      result: { outcome: "prepared" },
      promotionReceipt: { status: "prepared", candidateId: "CAND-1", reviewId: "REV-1", targetMemoryType: "semantic" }
    });
  }, 120_000);

  it("retries a receipt commit and keeps the verified low-sensitivity binding", async () => {
    const taskQueue = `dipole-agent-memory-promotion-commit-${Date.now()}`;
    const expiresAt = new Date(Date.now() + 10 * 60 * 1000).toISOString();
    const committedReceipts: string[] = [];
    const activities: AgentTaskWorkerActivities & Pick<AgentMemoryPromotionActivities, "commitPreparedAgentMemoryPromotion"> = {
      ...foundationAgentTaskActivities,
      async admitAgentTask(input) {
        return { taskId: input.taskId, runId: "RUN-COMMIT-1", runStatus: "running" };
      },
      async executeAgentTaskStep() {
        return { kind: "complete", output: { outcome: "committed" } };
      },
      async commitPreparedAgentMemoryPromotion(input) {
        committedReceipts.push(input.receipt.receiptSha256);
        if (committedReceipts.length === 1) throw new Error("transient Core receipt commit failure");
        return {
          memoryId: "MEM-COMMIT-1", memoryType: "semantic", status: "active",
          receiptSha256: input.receipt.receiptSha256,
          provenance: { sourceType: "memory_candidate", sourceId: input.receipt.candidateId, sequence: input.receipt.reviewId }
        };
      }
    };
    const worker = await Worker.create({
      connection: env.nativeConnection,
      ...(env.namespace === undefined ? {} : { namespace: env.namespace }),
      taskQueue,
      workflowsPath: new URL("./agent-task-workflow.ts", import.meta.url).pathname,
      activities
    });
    const result = await worker.runUntil(() => env.client.workflow.execute("agentTaskWorkflow", {
      taskQueue,
      workflowId: `dipole-agent-task/memory-promotion-commit-${Date.now()}`,
      args: [{
        taskId: "TASK-MEM-COMMIT", goal: "commit reviewed memory promotion",
        memoryPromotion: {
          tenantId: "dipole", principalUserId: "U100", agentId: "UAI", taskId: "TASK-MEM-COMMIT", runId: "RUN-COMMIT-1",
          candidateId: "CAND-1", candidateSha256: "a".repeat(64), reviewId: "REV-1", policyVersion: "memory-v1",
          candidateMemoryType: "observational", targetMemoryType: "semantic", expiresAt, commit: true
        },
        admission: {
          tenantId: "dipole", principalUserId: "U100", agentId: "UAI",
          triggerType: "manual", triggerRef: "CAND-1", eventId: "EVENT-1", requestId: "REQ-1", traceId: "TRACE-1"
        }
      }]
    })) as { status: string; output?: { promotionReceipt?: { receiptSha256: string }; promotionCommit?: { memoryId: string; memoryType: string; status: string; receiptSha256: string } } };
    expect(result).toMatchObject({
      status: "completed",
      output: {
        promotionCommit: { memoryId: "MEM-COMMIT-1", memoryType: "semantic", status: "active" }
      }
    });
    expect(committedReceipts).toHaveLength(2);
    expect(new Set(committedReceipts)).toEqual(new Set([result.output!.promotionReceipt!.receiptSha256]));
    expect(result.output!.promotionCommit!.receiptSha256).toBe(result.output!.promotionReceipt!.receiptSha256);
  }, 120_000);

  it("finishes the persistent Run as failed when receipt commit exhausts its retries", async () => {
    const taskQueue = `dipole-agent-memory-promotion-commit-failure-${Date.now()}`;
    const expiresAt = new Date(Date.now() + 10 * 60 * 1000).toISOString();
    const finished: Array<{ runStatus: string; lastError: string }> = [];
    const activities: AgentTaskWorkerActivities & Pick<AgentMemoryPromotionActivities, "commitPreparedAgentMemoryPromotion"> = {
      ...foundationAgentTaskActivities,
      async admitAgentTask(input) {
        return { taskId: input.taskId, runId: "RUN-COMMIT-FAIL-1", runStatus: "running" };
      },
      async executeAgentTaskStep() {
        return { kind: "complete", output: { outcome: "would-commit" } };
      },
      async commitPreparedAgentMemoryPromotion() {
        throw new Error("Core receipt commit remains unavailable");
      },
      async finishAgentTask(input) {
        finished.push({ runStatus: input.runStatus, lastError: input.lastError });
      }
    };
    const worker = await Worker.create({
      connection: env.nativeConnection,
      ...(env.namespace === undefined ? {} : { namespace: env.namespace }),
      taskQueue,
      workflowsPath: new URL("./agent-task-workflow.ts", import.meta.url).pathname,
      activities
    });
    const result = await worker.runUntil(() => env.client.workflow.execute("agentTaskWorkflow", {
      taskQueue,
      workflowId: `dipole-agent-task/memory-promotion-commit-failure-${Date.now()}`,
      args: [{
        taskId: "TASK-MEM-COMMIT-FAIL", goal: "commit reviewed memory promotion",
        memoryPromotion: {
          tenantId: "dipole", principalUserId: "U100", agentId: "UAI", taskId: "TASK-MEM-COMMIT-FAIL", runId: "RUN-COMMIT-FAIL-1",
          candidateId: "CAND-1", candidateSha256: "a".repeat(64), reviewId: "REV-1", policyVersion: "memory-v1",
          candidateMemoryType: "observational", targetMemoryType: "semantic", expiresAt, commit: true
        },
        admission: {
          tenantId: "dipole", principalUserId: "U100", agentId: "UAI",
          triggerType: "manual", triggerRef: "CAND-1", eventId: "EVENT-1"
        }
      }]
    })) as { status: string; failure?: { message?: string } };

    expect(result.status).toBe("failed");
    expect(result.failure?.message).toContain("Core receipt commit remains unavailable");
    expect(finished).toEqual([{ runStatus: "failed", lastError: "Core receipt commit remains unavailable" }]);
  }, 120_000);
});
