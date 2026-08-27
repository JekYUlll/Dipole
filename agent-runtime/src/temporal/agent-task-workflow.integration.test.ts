import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { TestWorkflowEnvironment } from "@temporalio/testing";
import { Worker } from "@temporalio/worker";

import type { AgentTaskWorkerActivities } from "./agent-task-activities.js";
import { TemporalTaskClient } from "./temporal-task-client.js";

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
      async executeAgentTaskStep(input) {
        calls += 1;
        if (calls < 3) {
          throw new Error("transient activity failure");
        }
        if (input.resume?.kind !== "approval") {
          return { kind: "wait_approval", requestId: "approval-1", summary: "send digest", checkpoint: { phase: "ready" } };
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
    await waitForStatus(env, handle, "waiting_approval");
    expect(calls).toBe(3);

    workerOne.shutdown();
    await workerOneRun;

    const workerTwo = await createWorker(env, taskQueue, activities);
    const workerTwoRun = workerTwo.run();
    await waitForStatus(env, handle, "waiting_approval");
    await handle.signal("resolveTaskApproval", { requestId: "approval-1", decision: "approved" });
    await waitForStatus(env, handle, "completed");
    const result = await handle.result();
    workerTwo.shutdown();
    await workerTwoRun;
    expect(result).toMatchObject({
      taskId: "task-recovery-1", status: "completed",
      output: { artifactId: "A1", checkpoint: { phase: "ready" } }
    });
    expect(calls).toBe(4);
    expect(admissions).toBe(1);
    expect(finishAttempts).toBe(2);

    await expect(client.start({ taskId: "task-recovery-1", goal: "late replay" })).rejects.toThrow(/already started/i);
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
