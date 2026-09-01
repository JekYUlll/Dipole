import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { stat, writeFile } from "node:fs/promises";
import { dirname, isAbsolute, join, resolve } from "node:path";
import { TestWorkflowEnvironment } from "@temporalio/testing";
import { Worker } from "@temporalio/worker";

import type { AgentTaskFinishInput, AgentTaskWorkerActivities } from "./agent-task-activities.js";
import {
  TemporalMcpTaskClient,
  TemporalTaskClient,
  TemporalTaskControlClient,
  TemporalTaskWorkflowInspector
} from "./temporal-task-client.js";
import { TemporalMcpWorkflowExecutionCatalog } from "./mcp-workflow-envelope.js";
import type { TemporalMcpDispatchActivities } from "./mcp-dispatch-activity.js";
import { AgentTaskProjectionReconciler } from "../reconcile/agent-task-projection-reconciler.js";
import { CapabilityRegistry } from "../capabilities/registry.js";
import { ConversationListCapability } from "../capabilities/conversation-list.js";
import { ConversationReadCapability } from "../capabilities/conversation-read.js";
import { agentRunId, agentTaskId, discoveredConversationMarker, type AgentEvent } from "../events/shadow-processor.js";
import { ModelShadowPlanner } from "../models/model-shadow-planner.js";
import { ModelRouter, type ModelAuditStore, type ModelCallRecovery } from "../models/model-router.js";
import { createTemporalFaultReceipt } from "../evals/temporal-fault-receipt.js";
import { createTemporalReadStepActivities } from "./agent-task-read-activities.js";
import { AgentTaskControlError, AgentTaskControlService } from "../control/agent-task-control.js";
import { buildServer } from "../server.js";
import { createInteractiveMessageExecutor } from "../mcp/mcp-message-write-projection.js";
import type { AgentCapabilityRPCClient } from "../capabilities/agent-capability-rpc.js";
import * as grpc from "@grpc/grpc-js";

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
    const approvalDeadline = Date.now() + 5 * 60_000;
    let calls = 0;
    let admissions = 0;
    let finishAttempts = 0;
    let persistedTerminalWrites = 0;
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
        persistedTerminalWrites += 1;
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
              expiresAtUnixMs: approvalDeadline
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
    const receipt = createTemporalFaultReceipt({
      schemaVersion: "dipole.agent.temporal-fault-observation.v1",
      drillId: "worker_replacement_approval_resume",
      observedAt: new Date().toISOString(),
      workflow: { taskId: "task-recovery-1", runId: "run-recovery-1" },
      transitions: projections.map(({ workflowStatus, workflowRevision }) => ({ revision: workflowRevision, status: workflowStatus })),
      faults: { workerReplacements: 1, terminalWriteRetries: finishAttempts - persistedTerminalWrites },
      effects: {
        admissions, stepExecutions: calls, approvalRequests, approvalResolutions,
        terminalWriteAttempts: finishAttempts, terminalPersistedWrites: persistedTerminalWrites,
        inputSignalsRejected: 0, inputResumptions: 0
      }
    });
    expect(receipt).toMatchObject({ outcome: "eligible", failures: [] });
    await archiveTemporalFaultReceipt(receipt);

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

  it("keeps host route authority in history and resumes through the dedicated MCP Activity", async () => {
    const taskQueue = `dipole-agent-task-mcp-${Date.now()}`;
    const routeBinding = {
      routeId: "calendar-event-read",
      routeVersion: 3,
      routeManifestSha256: "a".repeat(64)
    };
    const activityInputs: unknown[] = [];
    const checkpoint = { durable: "MCP-CHECKPOINT-1" };
    const activities: AgentTaskWorkerActivities & TemporalMcpDispatchActivities = {
      async admitAgentTask(input) {
        return { taskId: input.taskId, runId: "RUN-MCP-1", runStatus: "running" };
      },
      async finishAgentTask() {},
      async projectAgentTaskState() {},
      async requestAgentTaskApproval() {},
      async resolveAgentTaskApproval() {},
      async executeAgentTaskStep() {
        throw new Error("generic Agent step must not receive an external MCP execution");
      },
      async executeMcpDispatch(input) {
        activityInputs.push(input);
        if (input.kind === "begin") {
          return {
            kind: "wait_input",
            requestId: "INPUT-MCP-1",
            prompt: "Choose calendar scope",
            form: {
              schemaVersion: "dipole.agent.elicitation.v1",
              fields: [{ id: "scope", label: "Scope", type: "select", required: true, options: ["today", "week"] }]
            },
            source: {
              kind: "mcp",
              serverId: "calendar.example",
              toolName: "calendar.read_event",
              invocationId: "c".repeat(64),
              trust: "untrusted"
            },
            expiresAtUnixMs: Date.now() + 60_000,
            checkpoint
          };
        }
        return { kind: "complete", output: { artifactId: "ARTIFACT-MCP-1" } };
      }
    };
    const workerOne = await createWorker(env, taskQueue, activities);
    const workerOneRun = workerOne.run();
    const tasks = new TemporalMcpTaskClient(
      env.client.workflow,
      taskQueue,
      new TemporalMcpWorkflowExecutionCatalog([routeBinding])
    );
    const controls = new TemporalTaskControlClient(env.client.workflow);
    const started = await tasks.start({
      taskId: "TASK-MCP-1",
      goal: "read one calendar event",
      routeId: routeBinding.routeId,
      arguments: { calendarId: "CAL-1", eventId: "EV-1" },
      admission: {
        tenantId: "dipole",
        principalUserId: "U100",
        agentId: "UAI",
        triggerType: "user_request",
        triggerRef: "CONV-1",
        eventId: "EVENT-MCP-1",
        requestId: "REQ-MCP-1",
        traceId: "TRACE-MCP-1"
      }
    });
    const handle = env.client.workflow.getHandle(started.workflowId);
    await waitForStatus(env, handle, "waiting_input");

    workerOne.shutdown();
    await workerOneRun;
    const workerTwo = await createWorker(env, taskQueue, activities);
    const workerTwoRun = workerTwo.run();
    await controls.provideInput("TASK-MCP-1", {
      requestId: "INPUT-MCP-1",
      value: { scope: "today" }
    });

    await expect(handle.result()).resolves.toMatchObject({
      status: "completed",
      output: { artifactId: "ARTIFACT-MCP-1" }
    });
    expect(activityInputs).toEqual([
      {
        kind: "begin",
        ...routeBinding,
        taskId: "TASK-MCP-1",
        runId: "RUN-MCP-1",
        principalUserId: "U100",
        arguments: { calendarId: "CAL-1", eventId: "EV-1" },
        requestId: "REQ-MCP-1",
        traceId: "TRACE-MCP-1"
      },
      {
        kind: "resume",
        checkpoint,
        resume: { kind: "input", requestId: "INPUT-MCP-1", value: { scope: "today" } }
      }
    ]);
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
    let terminalWrites = 0;
    const projections: Array<{ workflowStatus: string; workflowRevision: number }> = [];
    const activities: AgentTaskWorkerActivities = {
      async admitAgentTask(input) { return { taskId: input.taskId, runId: "run-input-1", runStatus: "running" }; },
      async finishAgentTask() { terminalWrites += 1; },
      async projectAgentTaskState(input) { projections.push(input); },
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
    const receipt = createTemporalFaultReceipt({
      schemaVersion: "dipole.agent.temporal-fault-observation.v1", drillId: "worker_replacement_input_resume", observedAt: new Date().toISOString(),
      workflow: { taskId: "task-input-1", runId: "run-input-1" },
      transitions: projections.map(({ workflowRevision, workflowStatus }) => ({ revision: workflowRevision, status: workflowStatus })),
      faults: { workerReplacements: 1, terminalWriteRetries: 0 },
      effects: { admissions: 1, stepExecutions: calls, approvalRequests: 0, approvalResolutions: 0, terminalWriteAttempts: terminalWrites, terminalPersistedWrites: terminalWrites, inputSignalsRejected: 2, inputResumptions: 1 }
    });
    expect(receipt).toMatchObject({ outcome: "eligible", failures: [] });
    await archiveTemporalFaultReceipt(receipt);
    workerTwo.shutdown();
    await workerTwoRun;
  }, 120_000);

  it("resolves an owner-bound approval through the Runtime HTTP control surface", async () => {
    const taskQueue = `dipole-agent-task-http-approval-${Date.now()}`;
    const taskId = "task-http-approval-1";
    const approval = {
      approvalId: "APR-HTTP-1", capabilityId: "message.bulk.send",
      resourceScope: { resourceType: "conversation", resourceId: "G1", actions: ["write"] },
      scopeSha256: "a".repeat(64), argumentsSha256: "b".repeat(64), nonceSha256: "c".repeat(64),
      expiresAtUnixMs: Date.now() + 60_000
    };
    const resolutions: unknown[] = [];
    const activities: AgentTaskWorkerActivities = {
      async admitAgentTask(input) {
        return { taskId: input.taskId, runId: "run-http-approval-1", runStatus: "running" };
      },
      async finishAgentTask() {},
      async projectAgentTaskState() {},
      async requestAgentTaskApproval() {},
      async resolveAgentTaskApproval(input) { resolutions.push(input); },
      async executeAgentTaskStep(input) {
        if (input.resume?.kind === "approval") return { kind: "complete", output: { action: "approved" } };
        return { kind: "wait_approval", requestId: "REQ-HTTP-1", summary: "Send the prepared digest", approval };
      }
    };
    const worker = await createWorker(env, taskQueue, activities);
    const workerRun = worker.run();
    const tasks = new TemporalTaskClient(env.client.workflow, taskQueue);
    const control = new AgentTaskControlService({
      async authorizeTaskControl(requestedTaskId, principalUserId) {
        if (requestedTaskId !== taskId || principalUserId !== "U100") {
          throw new AgentTaskControlError("not_found", "Agent Task control policy unavailable");
        }
        return { taskId, taskStatus: "waiting_approval" };
      }
    }, new TemporalTaskControlClient(env.client.workflow));
    const server = buildServer({ isReady: () => true }, { secret: "control-secret", service: control });
    const headers = {
      "x-dipole-caller-service": "dipole-gateway",
      "x-dipole-service-token": "control-secret",
      "x-dipole-principal-user-id": "U100",
      "x-request-id": "REQ-HTTP-1",
      "x-trace-id": "TRACE-HTTP-1"
    };

    try {
      const started = await tasks.start({ taskId, goal: "send prepared digest" });
      const handle = env.client.workflow.getHandle(started.workflowId);
      await waitForStatus(env, handle, "waiting_approval");

      const pending = await server.inject({ method: "GET", url: `/internal/v1/agent/tasks/${taskId}`, headers });
      expect(pending.statusCode).toBe(200);
      expect(pending.json()).toMatchObject({
        taskId, status: "waiting_approval", persistentStatus: "waiting_approval",
        pending: { kind: "approval", requestId: "REQ-HTTP-1", approvalId: approval.approvalId }
      });

      const foreign = await server.inject({
        method: "POST", url: `/internal/v1/agent/tasks/${taskId}/approvals/${approval.approvalId}`,
        headers: { ...headers, "x-dipole-principal-user-id": "U200" }, payload: { decision: "approved" }
      });
      expect(foreign.statusCode).toBe(404);

      const resolved = await server.inject({
        method: "POST", url: `/internal/v1/agent/tasks/${taskId}/approvals/${approval.approvalId}`,
        headers, payload: { decision: "approved" }
      });
      expect(resolved.statusCode).toBe(202);
      expect(resolved.json()).toEqual({ taskId, approvalId: approval.approvalId, status: "resolution_requested" });

      await expect(handle.result()).resolves.toMatchObject({ taskId, status: "completed", output: { action: "approved" } });
      expect(resolutions).toEqual([expect.objectContaining({
        taskId, approvalId: approval.approvalId, decision: "approved", actorUserId: "U100"
      })]);
    } finally {
      await server.close();
      worker.shutdown();
      await workerRun;
    }
  }, 120_000);

  it("persists an interactive message approval before its one-time durable execution", async () => {
    const taskQueue = `dipole-agent-interactive-message-${Date.now()}`;
    const event: AgentEvent = {
      eventId: "E-INTERACTIVE-MESSAGE", eventType: "agent.interactive.requested", aggregateId: "interactive:MESSAGE-1",
      occurredAt: new Date().toISOString(), payload: { content: "/send Deployment status recorded." }
    };
    const taskId = agentTaskId({ tenantId: "dipole", agentUuid: "UAI", triggerType: event.eventType, triggerRef: event.aggregateId });
    const runId = agentRunId(taskId, "dipole-agent", "active");
    const approvals: unknown[] = [];
    const resolutions: unknown[] = [];
    const executions: unknown[] = [];
    const read = createTemporalReadStepActivities({
      planner: { plan: async () => ({ summary: "unused", steps: [] }) },
      audit: { append: async () => undefined },
      registry: new CapabilityRegistry(),
      trajectory: { append: async () => undefined, claimStep: async () => ({ outcome: "claimed" as const, token: "unused" }), completeStep: async () => undefined, failStep: async () => undefined },
      runtimeMode: "active",
      contextResolver: {
        resolveMcpContext: async () => ({
          tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId, runId, mode: "active",
          permissions: ["message.write"],
          resourceScopes: [{ resourceType: "conversation", resourceId: "direct:U100:UAI", actions: ["write"] }],
          approvedCapabilities: ["message.system.send"], eventId: event.eventId
        })
      },
      interactiveMessage: {
        execute: async (input) => {
          executions.push(input);
          return JSON.stringify({ resourceId: "MSG-INTERACTIVE-1", commandId: "CMD-INTERACTIVE-1" });
        }
      },
      stepLeaseMs: 60_000
    });
    const activities: AgentTaskWorkerActivities = {
      async admitAgentTask(input) { return { taskId: input.taskId, runId, runStatus: "running" }; },
      async finishAgentTask() {},
      async projectAgentTaskState() {},
      async requestAgentTaskApproval(input) { approvals.push(input); },
      async resolveAgentTaskApproval(input) { resolutions.push(input); },
      executeAgentTaskStep: input => read.executeAgentTaskStep(input)
    };
    const worker = await createWorker(env, taskQueue, activities);
    const workerRun = worker.run();
    const tasks = new TemporalTaskClient(env.client.workflow, taskQueue);
    const controls = new TemporalTaskControlClient(env.client.workflow);

    try {
      const started = await tasks.start({
        taskId, goal: "send status", shadowEvent: event,
        admission: {
          tenantId: "dipole", principalUserId: "U100", agentId: "UAI",
          triggerType: event.eventType, triggerRef: event.aggregateId, eventId: event.eventId
        }
      });
      const handle = env.client.workflow.getHandle(started.workflowId);
      await waitForStatus(env, handle, "waiting_approval");
      expect(executions).toEqual([]);
      expect(approvals).toEqual([expect.objectContaining({
        taskId, runId, approval: expect.objectContaining({
          capabilityId: "message.system.send",
          resourceScope: { resourceType: "conversation", resourceId: "direct:U100:UAI", actions: ["write"] }
        })
      })]);
      const pending = await controls.query(taskId) as { pending: { requestId: string; approvalId: string } };
      await controls.resolveApproval(taskId, {
        requestId: pending.pending.requestId, approvalId: pending.pending.approvalId, decision: "approved", actorUserId: "U100"
      });

      await expect(handle.result()).resolves.toMatchObject({
        status: "completed",
        output: { summary: "Sent one approved system message to your direct Agent conversation" }
      });
      expect(resolutions).toEqual([expect.objectContaining({ taskId, runId, decision: "approved", actorUserId: "U100" })]);
      expect(executions).toEqual([{ conversationId: "direct:U100:UAI", content: "Deployment status recorded." }]);
    } finally {
      worker.shutdown();
      await workerRun;
    }
  }, 120_000);

  it("retries an uncertain interactive message response with one command identity", async () => {
    const taskQueue = `dipole-agent-interactive-retry-${Date.now()}`;
    const event: AgentEvent = {
      eventId: "E-INTERACTIVE-RETRY", eventType: "agent.interactive.requested", aggregateId: "interactive:RETRY-1",
      occurredAt: new Date().toISOString(), payload: { content: "/send Retry-safe deployment status." }
    };
    const taskId = agentTaskId({ tenantId: "dipole", agentUuid: "UAI", triggerType: event.eventType, triggerRef: event.aggregateId });
    const runId = agentRunId(taskId, "dipole-agent", "active");
    const commandCalls: Array<{ invocationId: string }> = [];
    const persistedCommandIds = new Set<string>();
    const finishToolInvocation = vi.fn(async () => undefined);
    const client: Pick<AgentCapabilityRPCClient, "begin" | "finishToolInvocation" | "consumeApproval" | "resolveApprovalGrant" | "executeMessageCommand"> = {
      begin: async () => undefined,
      finishToolInvocation,
      consumeApproval: async () => undefined,
      resolveApprovalGrant: async (_taskId, _runId, capabilityId, resourceScope, argumentsSha256) => ({
        approvalId: "APR-INTERACTIVE-RETRY", capabilityId, resourceScope,
        scopeSha256: "fda03fe9202766c3e59c11b4b069749a400e50041a44d1d200ff41c64aefa8a5",
        argumentsSha256, nonceSha256: "e".repeat(64), expiresAtUnixMs: Date.now() + 60_000
      }),
      executeMessageCommand: async input => {
        commandCalls.push({ invocationId: input.invocationId });
        persistedCommandIds.add(input.invocationId);
        if (commandCalls.length === 1) {
          throw Object.assign(new Error("response lost after Message commit"), { code: grpc.status.UNAVAILABLE });
        }
        return {
          resourceType: "message", resourceId: "MSG-INTERACTIVE-RETRY-1",
          commandKind: "system_message", commandId: `tool:${input.invocationId}`
        };
      }
    };
    const read = createTemporalReadStepActivities({
      planner: { plan: async () => ({ summary: "unused", steps: [] }) },
      audit: { append: async () => undefined },
      registry: new CapabilityRegistry(),
      trajectory: { append: async () => undefined, claimStep: async () => ({ outcome: "claimed" as const, token: "unused" }), completeStep: async () => undefined, failStep: async () => undefined },
      runtimeMode: "active",
      contextResolver: {
        resolveMcpContext: async () => ({
          tenantId: "dipole", principalUuid: "U100", agentUuid: "UAI", taskId, runId, mode: "active",
          permissions: ["message.write"],
          resourceScopes: [{ resourceType: "conversation", resourceId: "direct:U100:UAI", actions: ["write"] }],
          approvedCapabilities: ["message.system.send"], eventId: event.eventId
        })
      },
      interactiveMessage: createInteractiveMessageExecutor(client),
      stepLeaseMs: 60_000
    });
    const activities: AgentTaskWorkerActivities = {
      async admitAgentTask(input) { return { taskId: input.taskId, runId, runStatus: "running" }; },
      async finishAgentTask() {},
      async projectAgentTaskState() {},
      async requestAgentTaskApproval() {},
      async resolveAgentTaskApproval() {},
      executeAgentTaskStep: input => read.executeAgentTaskStep(input)
    };
    const worker = await createWorker(env, taskQueue, activities);
    const workerRun = worker.run();
    const tasks = new TemporalTaskClient(env.client.workflow, taskQueue);
    const controls = new TemporalTaskControlClient(env.client.workflow);

    try {
      const started = await tasks.start({
        taskId, goal: "send retry-safe status", shadowEvent: event,
        admission: {
          tenantId: "dipole", principalUserId: "U100", agentId: "UAI",
          triggerType: event.eventType, triggerRef: event.aggregateId, eventId: event.eventId
        }
      });
      const handle = env.client.workflow.getHandle(started.workflowId);
      await waitForStatus(env, handle, "waiting_approval");
      const pending = await controls.query(taskId) as { pending: { requestId: string; approvalId: string } };
      await controls.resolveApproval(taskId, {
        requestId: pending.pending.requestId, approvalId: pending.pending.approvalId,
        decision: "approved", actorUserId: "U100"
      });

      await expect(handle.result()).resolves.toMatchObject({ status: "completed" });
      expect(commandCalls).toHaveLength(2);
      expect(commandCalls[0]!.invocationId).toBe(commandCalls[1]!.invocationId);
      expect(persistedCommandIds).toEqual(new Set([commandCalls[0]!.invocationId]));
      expect(finishToolInvocation).toHaveBeenCalledOnce();
      expect(finishToolInvocation).toHaveBeenCalledWith(expect.objectContaining({ status: "completed" }));
    } finally {
      worker.shutdown();
      await workerRun;
    }
  }, 120_000);

  it("cancels an interactive write after one denied approval and ignores replayed signals", async () => {
    const taskQueue = `dipole-agent-interactive-denied-${Date.now()}`;
    const finishes: AgentTaskFinishInput[] = [];
    const resolutions: unknown[] = [];
    const writeExecutions: unknown[] = [];
    const activities: AgentTaskWorkerActivities = {
      async admitAgentTask(input) { return { taskId: input.taskId, runId: "run-interactive-denied-1", runStatus: "running" }; },
      async finishAgentTask(input) { finishes.push(input); },
      async projectAgentTaskState() {},
      async requestAgentTaskApproval() {},
      async resolveAgentTaskApproval(input) { resolutions.push(input); },
      async executeAgentTaskStep(input) {
        if (input.resume?.kind === "approval") {
          writeExecutions.push(input.resume);
          return { kind: "complete", output: { action: "approved" } };
        }
        return {
          kind: "wait_approval", requestId: "REQ-DENIED-1", summary: "Send the prepared digest",
          approval: {
            approvalId: "APR-DENIED-1", capabilityId: "message.system.send",
            resourceScope: { resourceType: "conversation", resourceId: "direct:U100:UAI", actions: ["write"] },
            scopeSha256: "a".repeat(64), argumentsSha256: "b".repeat(64), nonceSha256: "c".repeat(64),
            expiresAtUnixMs: Date.now() + 60_000
          }
        };
      }
    };
    const worker = await createWorker(env, taskQueue, activities);
    const workerRun = worker.run();
    const tasks = new TemporalTaskClient(env.client.workflow, taskQueue);
    const controls = new TemporalTaskControlClient(env.client.workflow);

    try {
      const started = await tasks.start({ taskId: "task-interactive-denied-1", goal: "send prepared digest" });
      const handle = env.client.workflow.getHandle(started.workflowId);
      await waitForStatus(env, handle, "waiting_approval");
      const pending = await controls.query("task-interactive-denied-1") as { pending: { requestId: string; approvalId: string } };
      const decision = {
        requestId: pending.pending.requestId, approvalId: pending.pending.approvalId,
        decision: "denied" as const, actorUserId: "U100"
      };

      await Promise.all([
        controls.resolveApproval("task-interactive-denied-1", decision),
        controls.resolveApproval("task-interactive-denied-1", decision)
      ]);

      await expect(handle.result()).resolves.toMatchObject({
        status: "cancelled", cancellation: { reason: "approval_denied", requestId: "REQ-DENIED-1" }
      });
      expect(resolutions).toEqual([expect.objectContaining({
        approvalId: "APR-DENIED-1", decision: "denied", actorUserId: "U100"
      })]);
      expect(writeExecutions).toEqual([]);
      expect(finishes).toEqual([expect.objectContaining({ runStatus: "cancelled", lastError: "approval_denied" })]);
    } finally {
      worker.shutdown();
      await workerRun;
    }
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
    let authorizationAudits = 0;
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
      recordAuthorization: async () => { authorizationAudits += 1; },
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
    expect(authorizationAudits).toBe(1);
  }, 120_000);

  it("pauses the read path for owner confirmation and reads only the confirmed conversation", async () => {
    const taskQueue = `dipole-agent-read-scope-resume-${Date.now()}`;
    const drill = readScopeDrill({ suffix: "RESUME" });
    const worker = await createWorker(env, taskQueue, drill.activities);
    const workerRun = worker.run();
    const controls = new TemporalTaskControlClient(env.client.workflow);
    const started = await drill.start(new TemporalTaskClient(env.client.workflow, taskQueue));
    const handle = env.client.workflow.getHandle(started.workflowId);

    await waitForStatus(env, handle, "waiting_input");
    const paused = await controls.query(drill.taskId) as {
      pending?: { requestId: string; form?: { fields: ReadonlyArray<{ id: string; options?: readonly string[] }> } };
    };
    const requestId = paused.pending?.requestId ?? "";
    expect(requestId).toMatch(/^input:[a-f0-9]{58}$/);
    expect(paused.pending?.form?.fields[0]).toMatchObject({ id: "conversation", options: [...drill.candidates] });
    expect(drill.conversationReads).toEqual([]);

    await controls.provideInput(drill.taskId, { requestId: "input:forged", value: { conversation: drill.candidates[1] } });
    await env.sleep(100);
    await expect(controls.query(drill.taskId)).resolves.toMatchObject({ status: "waiting_input" });

    await controls.provideInput(drill.taskId, { requestId, value: { conversation: drill.candidates[1] } });
    await expect(handle.result()).resolves.toMatchObject({
      status: "completed", output: { summary: "confirmed digest", stepCount: 2 }
    });
    expect(drill.conversationReads).toEqual([drill.candidates[1]]);
    expect(drill.counters.stepExecutions).toBe(2);

    const receipt = createTemporalFaultReceipt(drill.observation("read_scope_confirmation_resume", {
      inputSignalsRejected: 1, inputResumptions: 1, confirmedConversationId: drill.candidates[1]
    }));
    expect(receipt).toMatchObject({ outcome: "eligible", failures: [] });
    await archiveTemporalFaultReceipt(receipt);
    worker.shutdown();
    await workerRun;
  }, 120_000);

  it("leaves the read unauthorised when the owner declines the confirmation", async () => {
    const taskQueue = `dipole-agent-read-scope-declined-${Date.now()}`;
    const drill = readScopeDrill({ suffix: "DECLINED" });
    const worker = await createWorker(env, taskQueue, drill.activities);
    const workerRun = worker.run();
    const controls = new TemporalTaskControlClient(env.client.workflow);
    const started = await drill.start(new TemporalTaskClient(env.client.workflow, taskQueue));
    const handle = env.client.workflow.getHandle(started.workflowId);

    await waitForStatus(env, handle, "waiting_input");
    await controls.cancel(drill.taskId, "user_cancelled");
    await expect(handle.result()).resolves.toMatchObject({
      status: "cancelled", cancellation: { reason: "user_cancelled" }
    });
    expect(drill.conversationReads).toEqual([]);
    expect(drill.counters.stepExecutions).toBe(1);

    const receipt = createTemporalFaultReceipt(drill.observation("read_scope_confirmation_declined", {
      inputSignalsRejected: 0, inputResumptions: 0, cancellation: { reason: "user_cancelled" }
    }));
    expect(receipt).toMatchObject({ outcome: "eligible", failures: [] });
    await archiveTemporalFaultReceipt(receipt);
    worker.shutdown();
    await workerRun;
  }, 120_000);

  it("leaves the read unauthorised when the confirmation deadline passes unanswered", async () => {
    const taskQueue = `dipole-agent-read-scope-expired-${Date.now()}`;
    const drill = readScopeDrill({ suffix: "EXPIRED", confirmationTtlMs: 2_000 });
    const worker = await createWorker(env, taskQueue, drill.activities);
    const workerRun = worker.run();
    const started = await drill.start(new TemporalTaskClient(env.client.workflow, taskQueue));

    await expect(env.client.workflow.getHandle(started.workflowId).result()).resolves.toMatchObject({
      status: "cancelled", cancellation: { reason: "input_expired" }
    });
    expect(drill.conversationReads).toEqual([]);
    expect(drill.counters.stepExecutions).toBe(1);

    const receipt = createTemporalFaultReceipt(drill.observation("read_scope_confirmation_expired", {
      inputSignalsRejected: 0, inputResumptions: 0, cancellation: { reason: "input_expired" }
    }));
    expect(receipt).toMatchObject({ outcome: "eligible", failures: [] });
    await archiveTemporalFaultReceipt(receipt);
    worker.shutdown();
    await workerRun;
  }, 120_000);
});

// Drives the production read Activity behind the Workflow so a drill observes the
// real pause, the real resume binding, and every conversation the Run actually read.
function readScopeDrill(options: { readonly suffix: string; readonly confirmationTtlMs?: number }) {
  const event: AgentEvent = {
    eventId: `E-READ-SCOPE-${options.suffix}`, eventType: "message.direct.created", aggregateId: `M-READ-SCOPE-${options.suffix}`,
    occurredAt: "2026-09-01T08:00:00.000Z", payload: { content: "untrusted evidence" }
  };
  const taskId = agentTaskId({ tenantId: "dipole", agentUuid: "UAI", triggerType: event.eventType, triggerRef: event.aggregateId });
  const candidates = ["group:G1", "direct:U100:U200", "group:G3"] as const;
  const counters = { admissions: 0, stepExecutions: 0, terminalWrites: 0 };
  const conversationReads: string[] = [];
  const transitions: Array<{ revision: number; status: string }> = [];
  const router = {
    async generate(request: { stage?: string }) {
      return request.stage === "synthesis"
        ? { output: { summary: "confirmed digest" }, route: "gateway/primary", attempts: 1, usage: { inputTokens: 30, outputTokens: 10 } }
        : {
          output: { summary: "read newest conversation", steps: [
            { capabilityId: "conversation.list", input: { limit: 10 } },
            { capabilityId: "conversation.read", input: { conversationId: discoveredConversationMarker, limit: 10 } }
          ] },
          route: "gateway/primary", attempts: 1, usage: { inputTokens: 20, outputTokens: 8 }
        };
    }
  };
  const registry = new CapabilityRegistry();
  registry.register(new ConversationListCapability({
    listConversations: async () => candidates.map((conversationKey, index) => ({
      conversationKey, targetId: `T${index}`, targetType: 2, lastMessageId: `M${index}`, lastMessageSeq: "1",
      lastMessagePreview: "hello", lastMessageAtUnixMs: "1787817600000", readSeq: "0", unreadCount: 1
    }))
  }));
  registry.register(new ConversationReadCapability({
    readConversation: async (_context: unknown, conversationId: string) => {
      conversationReads.push(conversationId);
      return { found: true, reason: "", targetId: "T1", targetType: 2, messages: [] };
    }
  }));
  const completedSteps = new Set<number>();
  const trajectory = {
    append: async () => undefined,
    claimStep: async (_taskId: string, stepNo: number) => completedSteps.has(stepNo)
      ? { outcome: "completed" as const }
      : { outcome: "claimed" as const, token: `TOKEN-${stepNo}` },
    recordAuthorization: async () => undefined,
    completeStep: async (_taskId: string, stepNo: number) => { completedSteps.add(stepNo); },
    failStep: async () => undefined
  };
  const read = createTemporalReadStepActivities({
    planner: new ModelShadowPlanner(router as unknown as ModelRouter, ["conversation.list", "conversation.read"]),
    audit: trajectory, registry, trajectory, stepLeaseMs: 60_000,
    ...(options.confirmationTtlMs === undefined ? {} : { readScopeConfirmationTtlMs: options.confirmationTtlMs })
  });
  const activities: AgentTaskWorkerActivities = {
    async admitAgentTask(input) {
      counters.admissions += 1;
      return { taskId: input.taskId, runId: agentRunId(input.taskId), runStatus: "running" };
    },
    async finishAgentTask() { counters.terminalWrites += 1; },
    async projectAgentTaskState(input) { transitions.push({ revision: input.workflowRevision, status: input.workflowStatus }); },
    async requestAgentTaskApproval() {},
    async resolveAgentTaskApproval() {},
    async executeAgentTaskStep(input) {
      counters.stepExecutions += 1;
      return read.executeAgentTaskStep(input);
    }
  };
  return {
    taskId, candidates, counters, conversationReads, transitions, activities,
    start: async (tasks: TemporalTaskClient) => tasks.start({
      taskId, goal: "observe", shadowEvent: event,
      admission: {
        tenantId: "dipole", principalUserId: "U100", agentId: "UAI",
        triggerType: event.eventType, triggerRef: event.aggregateId, eventId: event.eventId
      }
    }),
    observation: (
      drillId: string,
      input: {
        inputSignalsRejected: number; inputResumptions: number;
        confirmedConversationId?: string; cancellation?: { reason: string };
      }
    ) => ({
      schemaVersion: "dipole.agent.temporal-fault-observation.v1", drillId, observedAt: new Date().toISOString(),
      workflow: { taskId, runId: agentRunId(taskId) },
      transitions: transitions.map(({ revision, status }) => ({ revision, status })),
      faults: { workerReplacements: 0, terminalWriteRetries: 0 },
      effects: {
        admissions: counters.admissions, stepExecutions: counters.stepExecutions,
        approvalRequests: 0, approvalResolutions: 0,
        terminalWriteAttempts: counters.terminalWrites, terminalPersistedWrites: counters.terminalWrites,
        inputSignalsRejected: input.inputSignalsRejected, inputResumptions: input.inputResumptions,
        conversationReads: conversationReads.length,
        unconfirmedConversationReads: conversationReads.filter((id) => id !== input.confirmedConversationId).length
      },
      ...(input.cancellation === undefined ? {} : { cancellation: input.cancellation })
    })
  };
}

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

async function archiveTemporalFaultReceipt(receipt: ReturnType<typeof createTemporalFaultReceipt>): Promise<void> {
  const configuredDirectory = process.env.DIPOLE_AGENT_TEMPORAL_FAULT_EVIDENCE_DIR?.trim();
  if (!configuredDirectory) return;
  if (!isAbsolute(configuredDirectory)) throw new Error("Temporal fault evidence directory must be absolute");

  const directory = resolve(configuredDirectory);
  const metadata = await stat(directory);
  if (!metadata.isDirectory()) throw new Error("Temporal fault evidence directory must be a directory");

  const outputPath = resolve(directory, `${receipt.drillId}.json`);
  if (dirname(outputPath) !== directory) throw new Error("Temporal fault evidence path is invalid");
  await writeFile(outputPath, `${JSON.stringify(receipt, null, 2)}\n`, { encoding: "utf8", mode: 0o600, flag: "wx" });
}
