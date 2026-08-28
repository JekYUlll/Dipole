import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { TestWorkflowEnvironment } from "@temporalio/testing";
import { Worker } from "@temporalio/worker";

import type { AgentTaskFinishInput, AgentTaskWorkerActivities } from "./agent-task-activities.js";
import { TemporalTaskClient, TemporalTaskControlClient, TemporalTaskWorkflowInspector } from "./temporal-task-client.js";
import { AgentTaskProjectionReconciler } from "../reconcile/agent-task-projection-reconciler.js";
import { CapabilityRegistry } from "../capabilities/registry.js";
import { ConversationListCapability } from "../capabilities/conversation-list.js";
import { agentRunId, agentTaskId, type AgentEvent } from "../events/shadow-processor.js";
import { ModelShadowPlanner } from "../models/model-shadow-planner.js";
import { ModelRouter, type ModelAuditStore, type ModelCallRecovery } from "../models/model-router.js";
import { createTemporalReadStepActivities } from "./agent-task-read-activities.js";

const integrationEnabled = process.env.DIPOLE_AGENT_TEMPORAL_INTEGRATION === "true";

describe.skipIf(!integrationEnabled)("Agent Task Temporal integration", () => {
  let env: TestWorkflowEnvironment;

  beforeAll(async () => {
    env = await TestWorkflowEnvironment.createLocal();
  }, 120_000);

  afterAll(async () => {
    await env?.teardown();
  });

  it("retries Activities, converges duplicate starts, and resumes after Worker replacement", async () => {
    const taskQueue = `dipole-agent-task-test-${Date.now()}`;
    let calls = 0;
    let admissions = 0;
    let finishAttempts = 0;
    let approvalRequests = 0;
    let approvalResolutions = 0;
    const projections: Array<{ workflowStatus: string; workflowRevision: number; workflowId: string; workflowRunId: string }> = [];
    const activities: AgentTaskWorkerActivities = {
      async admitAgentTask(input) {
        admissions += 1;
        return { taskId: input.taskId, runId: "run-recovery-1", runStatus: "running" };
      },
      async finishAgentTask(input) {
        finishAttempts += 1;
        expect(input).toMatchObject({
          taskId: "task-recovery-1", runId: "run-recovery-1", runStatus: "completed", lastError: ""
        });
        if (finishAttempts === 1) {
          throw new Error("transient terminal write failure");
        }
      },
      async projectAgentTaskState(input) {
        projections.push(input);
      },
      async requestAgentTaskApproval(input) {
        approvalRequests += 1;
        expect(input.approval.approvalId).toBe("APR-1");
      },
      async resolveAgentTaskApproval(input) {
        approvalResolutions += 1;
        expect(input).toMatchObject({ approvalId: "APR-1", actorUserId: "U100", decision: "approved" });
      },
      async executeAgentTaskStep(input) {
        calls += 1;
        if (calls < 3) {
          throw new Error("transient activity failure");
        }
        if (input.resume?.kind !== "approval") {
          return {
            kind: "wait_approval", requestId: "approval-1", summary: "send digest", checkpoint: { phase: "ready" },
            approval: {
              approvalId: "APR-1", capabilityId: "message.bulk.send",
              resourceScope: { resourceType: "conversation", resourceId: "G1", actions: ["write"] },
              scopeSha256: "a".repeat(64), argumentsSha256: "b".repeat(64), nonceSha256: "c".repeat(64),
              expiresAtUnixMs: Date.UTC(2026, 7, 28)
            }
          };
        }
        return { kind: "complete", output: { artifactId: "A1", checkpoint: input.checkpoint } };
      }
    };
    const workerOne = await createWorker(env, taskQueue, activities);
    const workerOneRun = workerOne.run();
    const client = new TemporalTaskClient(env.client.workflow, taskQueue);

    const first = await client.start({ taskId: "task-recovery-1", goal: "publish a digest" });
    const duplicate = await client.start({ taskId: "task-recovery-1", goal: "ignored duplicate payload" });
    expect(duplicate.runId).toBe(first.runId);

    const handle = env.client.workflow.getHandle(first.workflowId);
    const controls = new TemporalTaskControlClient(env.client.workflow);
    await waitForStatus(env, handle, "waiting_approval");
    await expect(controls.query("task-recovery-1")).resolves.toMatchObject({
      taskId: "task-recovery-1", status: "waiting_approval",
      pending: { kind: "approval", requestId: "approval-1", approvalId: "APR-1" }
    });
    expect(calls).toBe(3);

    workerOne.shutdown();
    await workerOneRun;

    const workerTwo = await createWorker(env, taskQueue, activities);
    const workerTwoRun = workerTwo.run();
    await waitForStatus(env, handle, "waiting_approval");
    await controls.resolveApproval("task-recovery-1", {
      requestId: "approval-1", approvalId: "APR-1", decision: "approved", actorUserId: "U100"
    });
    await waitForStatus(env, handle, "completed");
    const result = await handle.result();
    expect(result).toMatchObject({
      taskId: "task-recovery-1", status: "completed",
      output: { artifactId: "A1", checkpoint: { phase: "ready" } }
    });
    expect(calls).toBe(4);
    expect(admissions).toBe(1);
    expect(finishAttempts).toBe(2);
    expect(approvalRequests).toBe(1);
    expect(approvalResolutions).toBe(1);
    expect(projections.map(({ workflowStatus, workflowRevision }) => ({ workflowStatus, workflowRevision }))).toEqual([
      { workflowStatus: "running", workflowRevision: 1 },
      { workflowStatus: "waiting_approval", workflowRevision: 2 },
      { workflowStatus: "running", workflowRevision: 3 },
      { workflowStatus: "completed", workflowRevision: 4 }
    ]);
    expect(projections.every((projection) => projection.workflowId === first.workflowId && projection.workflowRunId === first.runId)).toBe(true);

    const report = await new AgentTaskProjectionReconciler({
      list: async () => ({ tasks: [{ taskId: "task-recovery-1", workflow: {
        workflowId: first.workflowId, workflowRunId: first.runId!, workflowStatus: "completed", workflowRevision: 4
      } }], nextCursor: "" })
    }, new TemporalTaskWorkflowInspector(env.client.workflow)).run({ pageSize: 10, maxExamples: 10 });
    expect(report).toMatchObject({ consistent: true, scanned: 1, outcomes: { match: 1 } });

    await expect(client.start({ taskId: "task-recovery-1", goal: "late replay" })).rejects.toThrow(/already started/i);
    workerTwo.shutdown();
    await workerTwoRun;
  }, 120_000);

  it("cancels a waiting Workflow through the stable Task control adapter", async () => {
    const taskQueue = `dipole-agent-task-cancel-${Date.now()}`;
    const terminalWrites: unknown[] = [];
    const projections: Array<{ workflowStatus: string; workflowRevision: number }> = [];
    const activities: AgentTaskWorkerActivities = {
      async admitAgentTask(input) {
        return { taskId: input.taskId, runId: "run-cancel-1", runStatus: "running" };
      },
      async finishAgentTask(input) {
        terminalWrites.push(input);
      },
      async projectAgentTaskState(input) {
        projections.push(input);
      },
      async requestAgentTaskApproval() {},
      async resolveAgentTaskApproval() {},
      async executeAgentTaskStep() {
        return { kind: "wait_input", requestId: "INPUT-1", prompt: "choose scope", expiresAtUnixMs: Date.now() + 60_000, form: {
          schemaVersion: "dipole.agent.elicitation.v1", fields: [{ id: "scope", label: "Scope", type: "select", required: true, options: ["today", "week"] }]
        } };
      }
    };
    const worker = await createWorker(env, taskQueue, activities);
    const workerRun = worker.run();
    const tasks = new TemporalTaskClient(env.client.workflow, taskQueue);
    const controls = new TemporalTaskControlClient(env.client.workflow);
    const started = await tasks.start({ taskId: "task-cancel-1", goal: "wait for scope" });
    const handle = env.client.workflow.getHandle(started.workflowId);
    await waitForStatus(env, handle, "waiting_input");

    await controls.cancel("task-cancel-1", "user_cancelled");
    await waitForStatus(env, handle, "cancelled");
    await expect(handle.result()).resolves.toMatchObject({
      taskId: "task-cancel-1", status: "cancelled", cancellation: { reason: "user_cancelled" }
    });
    expect(terminalWrites).toEqual([expect.objectContaining({
      taskId: "task-cancel-1", runId: "run-cancel-1", runStatus: "cancelled", lastError: "user_cancelled"
    })]);
    expect(projections).toEqual([
      expect.objectContaining({ workflowStatus: "running", workflowRevision: 1 }),
      expect.objectContaining({ workflowStatus: "waiting_input", workflowRevision: 2 }),
      expect.objectContaining({ workflowStatus: "cancelled", workflowRevision: 3 })
    ]);
    worker.shutdown();
    await workerRun;
  }, 120_000);

  it("keeps invalid input pending and resumes from an exact durable Elicitation Signal", async () => {
    const taskQueue = `dipole-agent-task-input-${Date.now()}`;
    let calls = 0;
    const activities: AgentTaskWorkerActivities = {
      async admitAgentTask(input) { return { taskId: input.taskId, runId: "run-input-1", runStatus: "running" }; },
      async finishAgentTask() {},
      async projectAgentTaskState() {},
      async requestAgentTaskApproval() {},
      async resolveAgentTaskApproval() {},
      async executeAgentTaskStep(input) {
        calls += 1;
        if (input.resume?.kind === "input") return { kind: "complete", output: { scope: input.resume.value.scope } };
        return { kind: "wait_input", requestId: "INPUT-1", prompt: "Choose scope", expiresAtUnixMs: Date.now() + 60_000, form: {
          schemaVersion: "dipole.agent.elicitation.v1", fields: [{ id: "scope", label: "Scope", type: "select", required: true, options: ["today", "week"] }]
        } };
      }
    };
    const workerOne = await createWorker(env, taskQueue, activities);
    const workerOneRun = workerOne.run();
    const tasks = new TemporalTaskClient(env.client.workflow, taskQueue);
    const controls = new TemporalTaskControlClient(env.client.workflow);
    const started = await tasks.start({ taskId: "task-input-1", goal: "choose scope" });
    const handle = env.client.workflow.getHandle(started.workflowId);
    await waitForStatus(env, handle, "waiting_input");
    await controls.provideInput("task-input-1", { requestId: "INPUT-1", value: { scope: "month" } });
    await env.sleep(100);
    await expect(controls.query("task-input-1")).resolves.toMatchObject({
      status: "waiting_input", pending: { requestId: "INPUT-1", source: { kind: "agent" } }
    });

    workerOne.shutdown();
    await workerOneRun;
    const workerTwo = await createWorker(env, taskQueue, activities);
    const workerTwoRun = workerTwo.run();
    await controls.provideInput("task-input-1", { requestId: "INPUT-OLD", value: { scope: "today" } });
    await controls.provideInput("task-input-1", { requestId: "INPUT-1", value: { scope: "today" } });
    await expect(handle.result()).resolves.toMatchObject({ status: "completed", output: { scope: "today" } });
    expect(calls).toBe(2);
    workerTwo.shutdown();
    await workerTwoRun;
  }, 120_000);

  it("cancels an unanswered durable input after its recorded deadline", async () => {
    const taskQueue = `dipole-agent-task-input-timeout-${Date.now()}`;
    const finishes: AgentTaskFinishInput[] = [];
    const activities: AgentTaskWorkerActivities = {
      async admitAgentTask(input) { return { taskId: input.taskId, runId: "run-input-timeout-1", runStatus: "running" }; },
      async finishAgentTask(input) { finishes.push(input); },
      async projectAgentTaskState() {},
      async requestAgentTaskApproval() {},
      async resolveAgentTaskApproval() {},
      async executeAgentTaskStep() {
        return { kind: "wait_input", requestId: "INPUT-1", prompt: "Choose scope", expiresAtUnixMs: Date.now() + 150, form: {
          schemaVersion: "dipole.agent.elicitation.v1", fields: [{ id: "scope", label: "Scope", type: "select", required: true, options: ["today"] }]
        } };
      }
    };
    const worker = await createWorker(env, taskQueue, activities);
    const workerRun = worker.run();
    const tasks = new TemporalTaskClient(env.client.workflow, taskQueue);
    const started = await tasks.start({ taskId: "task-input-timeout-1", goal: "choose scope" });
    await expect(env.client.workflow.getHandle(started.workflowId).result()).resolves.toMatchObject({
      status: "cancelled", cancellation: { reason: "input_expired", requestId: "INPUT-1" }
    });
    expect(finishes).toHaveLength(1);
    expect(finishes[0]).toMatchObject({ runStatus: "cancelled", lastError: "input_expired" });
    worker.shutdown();
    await workerRun;
  }, 120_000);

  it("fails a Task after its bounded Activity step budget", async () => {
    const taskQueue = `dipole-agent-task-limit-${Date.now()}`;
    let calls = 0;
    const terminalWrites: unknown[] = [];
    const activities: AgentTaskWorkerActivities = {
      async admitAgentTask(input) {
        return { taskId: input.taskId, runId: "run-step-limit-1", runStatus: "running" };
      },
      async finishAgentTask(input) {
        terminalWrites.push(input);
      },
      async projectAgentTaskState() {},
      async requestAgentTaskApproval() {},
      async resolveAgentTaskApproval() {},
      async executeAgentTaskStep() {
        calls += 1;
        return { kind: "continue", checkpoint: { calls } };
      }
    };
    const worker = await createWorker(env, taskQueue, activities);
    const result = await worker.runUntil(() => env.client.workflow.execute("agentTaskWorkflow", {
      taskQueue,
      workflowId: "dipole-agent-task/task-step-limit-1",
      args: [{ taskId: "task-step-limit-1", goal: "loop", maxSteps: 2 }]
    })) as { status: string; failure?: { message: string } };

    expect(calls).toBe(2);
    expect(result).toMatchObject({
      status: "failed", failure: { message: "Agent Task exceeded the 2 Activity step limit" }
    });
    expect(terminalWrites).toEqual([expect.objectContaining({
      taskId: "task-step-limit-1",
      runId: "run-step-limit-1",
      runStatus: "failed",
      lastError: "Agent Task exceeded the 2 Activity step limit"
    })]);
  }, 120_000);

  it("replays a completed model result and Step after an Activity post-effect failure", async () => {
    const taskQueue = `dipole-agent-task-read-${Date.now()}`;
    const event: AgentEvent = {
      eventId: "E-TEMPORAL-RECOVERY", eventType: "message.direct.created", aggregateId: "M-TEMPORAL-RECOVERY",
      occurredAt: "2026-08-27T08:00:00.000Z", payload: { content: "untrusted evidence" }
    };
    const taskId = agentTaskId({ tenantId: "dipole", agentUuid: "UAI", triggerType: event.eventType, triggerRef: event.aggregateId });
    let providerCalls = 0;
    let capabilityCalls = 0;
    let completedModel: ModelCallRecovery | undefined;
    const modelAudit: ModelAuditStore = {
      recover: async () => completedModel,
      reserve: async () => ({ runId: "MODEL-RUN-1", callId: "MODEL-CALL-1", callNo: 1, route: "primary" }),
      completeCall: async (reservation, output, usage) => {
        completedModel = { ...reservation, output, usage };
      },
      failCall: async () => undefined,
      completeRun: async () => undefined,
      failRun: async () => undefined,
      failTask: async () => undefined
    };
    const router = new ModelRouter({
      generate: async () => {
        providerCalls += 1;
        return {
          output: { summary: "read", steps: [{ capabilityId: "conversation.list", input: { limit: 10 } }] },
          usage: { inputTokens: 20, outputTokens: 8 }, finishReason: "stop"
        };
      }
    }, ["primary"], { maxCalls: 1, totalTimeoutMs: 5000, maxOutputTokensPerCall: 128 }, undefined, modelAudit);
    const registry = new CapabilityRegistry();
    registry.register(new ConversationListCapability({
      listConversations: async () => {
        capabilityCalls += 1;
        return [];
      }
    }));
    let stepCompleted = false;
    const trajectory = {
      append: async () => undefined,
      claimStep: async () => stepCompleted
        ? { outcome: "completed" as const }
        : { outcome: "claimed" as const, token: "TOKEN-1" },
      completeStep: async () => { stepCompleted = true; },
      failStep: async () => undefined
    };
    const read = createTemporalReadStepActivities({
      planner: new ModelShadowPlanner(router, ["conversation.list"]),
      audit: trajectory, registry, trajectory, stepLeaseMs: 60_000
    });
    let activityAttempts = 0;
    const activities: AgentTaskWorkerActivities = {
      async admitAgentTask(input) {
        return { taskId: input.taskId, runId: agentRunId(input.taskId), runStatus: "running" };
      },
      async finishAgentTask() {},
      async projectAgentTaskState() {},
      async requestAgentTaskApproval() {},
      async resolveAgentTaskApproval() {},
      async executeAgentTaskStep(input) {
        activityAttempts += 1;
        const result = await read.executeAgentTaskStep(input);
        if (activityAttempts === 1) {
          throw new Error("lost Activity completion acknowledgement");
        }
        return result;
      }
    };
    const worker = await createWorker(env, taskQueue, activities);
    const result = await worker.runUntil(() => env.client.workflow.execute("agentTaskWorkflow", {
      taskQueue,
      workflowId: `dipole-agent-task/${taskId}`,
      args: [{
        taskId, goal: "observe", shadowEvent: event,
        admission: {
          tenantId: "dipole", principalUserId: "U100", agentId: "UAI",
          triggerType: event.eventType, triggerRef: event.aggregateId, eventId: event.eventId
        }
      }]
    })) as { status: string; output?: unknown };

    expect(result).toMatchObject({ status: "completed", output: { summary: "read", stepCount: 1 } });
    expect(activityAttempts).toBe(2);
    expect(providerCalls).toBe(1);
    expect(capabilityCalls).toBe(1);
  }, 120_000);
});

async function createWorker(
  env: TestWorkflowEnvironment,
  taskQueue: string,
  activities: AgentTaskWorkerActivities
): Promise<Worker> {
  return Worker.create({
    connection: env.nativeConnection,
    ...(env.namespace === undefined ? {} : { namespace: env.namespace }),
    taskQueue,
    workflowsPath: new URL("./agent-task-workflow.ts", import.meta.url).pathname,
    activities
  });
}

async function waitForStatus(
  env: TestWorkflowEnvironment,
  handle: ReturnType<TestWorkflowEnvironment["client"]["workflow"]["getHandle"]>,
  expected: string
): Promise<void> {
  let lastStatus = "unknown";
  for (let attempt = 0; attempt < 40; attempt += 1) {
    const state = await handle.query<{ status: string }>("taskState");
    lastStatus = state.status;
    if (state.status === expected) {
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error(`Workflow did not reach ${expected}; last status was ${lastStatus}`);
}
