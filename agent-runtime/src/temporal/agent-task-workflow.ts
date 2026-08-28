import {
  condition,
  defineQuery,
  defineSignal,
  patched,
  proxyActivities,
  setHandler,
  workflowInfo
} from "@temporalio/workflow";

import {
  createAgentTaskState,
  transitionAgentTask,
  type AgentTaskState
} from "../task/agent-task-state.js";
import type {
  AgentTaskActivities,
  AgentTaskDirective,
  AgentTaskFinishInput,
  AgentTaskLifecycleActivities,
  AgentTaskProjectionInput,
  AgentTaskRunBinding
} from "./agent-task-activities.js";
import type { AgentMemoryPromotionActivities } from "./agent-task-activities.js";
import type { AgentMemoryPromotionReceipt } from "../memory/agent-memory-promotion-receipt.js";
import type { AgentTaskWorkflowHistoryInput } from "./temporal-task-client.js";
import type { TemporalMcpDispatchActivities } from "./mcp-dispatch-activity.js";
import {
  createTemporalMcpBeginActivityInput,
  createTemporalMcpResumeActivityInput,
  validateTemporalMcpWorkflowExecution
} from "./mcp-workflow-envelope.js";

export const provideTaskInputSignal = defineSignal<[
  { requestId: string; value: unknown }
]>("provideTaskInput");
export const resolveTaskApprovalSignal = defineSignal<[
  { requestId: string; approvalId: string; decision: "approved" | "denied"; actorUserId: string }
]>("resolveTaskApproval");
export const cancelTaskSignal = defineSignal<[{ reason: string }]>("cancelTask");
export const taskStateQuery = defineQuery<AgentTaskState>("taskState");

const { executeAgentTaskStep } = proxyActivities<AgentTaskActivities>({
  startToCloseTimeout: "2 minutes",
  retry: {
    initialInterval: "1 second",
    backoffCoefficient: 2,
    maximumInterval: "30 seconds",
    maximumAttempts: 3
  }
});
const { executeMcpDispatch } = proxyActivities<TemporalMcpDispatchActivities>({
  startToCloseTimeout: "2 minutes",
  retry: {
    initialInterval: "1 second",
    backoffCoefficient: 2,
    maximumInterval: "30 seconds",
    maximumAttempts: 3
  }
});
const { admitAgentTask, finishAgentTask, projectAgentTaskState, requestAgentTaskApproval, resolveAgentTaskApproval } = proxyActivities<AgentTaskLifecycleActivities>({
  startToCloseTimeout: "30 seconds",
  retry: {
    initialInterval: "1 second",
    backoffCoefficient: 2,
    maximumInterval: "30 seconds",
    maximumAttempts: 3
  }
});
const { prepareAgentMemoryPromotion } = proxyActivities<AgentMemoryPromotionActivities>({
  startToCloseTimeout: "30 seconds",
  retry: { initialInterval: "1 second", backoffCoefficient: 2, maximumInterval: "10 seconds", maximumAttempts: 3 }
});

export async function agentTaskWorkflow(input: AgentTaskWorkflowHistoryInput): Promise<AgentTaskState> {
  let state = createAgentTaskState(input.taskId);
  let checkpoint: unknown;
  let step = 0;
  let approvalSignal: { requestId: string; approvalId: string; decision: "approved" | "denied"; actorUserId: string } | undefined;
  let projectedRevision = -1;
  const maxSteps = validMaxSteps(input.maxSteps);
  const mcpExecution = input.execution === undefined
    ? undefined
    : validateTemporalMcpWorkflowExecution(input.execution);
  if (mcpExecution !== undefined && input.admission === undefined) {
    throw new Error("External MCP Agent Task requires trusted admission");
  }
  const projectionEnabled = patched("agent-task-workflow-projection-v1");
  const workflow = workflowInfo();
  let promotionReceipt: AgentMemoryPromotionReceipt | undefined;

  setHandler(taskStateQuery, () => state);
  setHandler(provideTaskInputSignal, (signal) => {
    if (state.status !== "waiting_input" || state.pending?.kind !== "input" || state.pending.requestId !== signal.requestId) {
      return;
    }
    try {
      state = transitionAgentTask(state, { type: "provide_input", requestId: signal.requestId, value: signal.value });
    } catch {
      // Invalid or stale responses leave the durable request pending.
    }
  });
  setHandler(resolveTaskApprovalSignal, (signal) => {
    if (approvalSignal !== undefined || state.status !== "waiting_approval" || state.pending?.kind !== "approval" || state.pending.requestId !== signal.requestId || state.pending.approvalId !== signal.approvalId) {
      return;
    }
    approvalSignal = signal;
  });
  setHandler(cancelTaskSignal, (signal) => {
    if (!isTerminal(state)) {
      state = transitionAgentTask(state, { type: "cancel", reason: signal.reason });
    }
  });

  const binding = await admitAgentTask(input);
  assertRunBinding(input.taskId, binding);
  if (input.memoryPromotion !== undefined) {
    promotionReceipt = await prepareAgentMemoryPromotion({
      ...input.memoryPromotion,
      createdAt: new Date().toISOString()
    });
  }
  if (binding.runStatus === "completed") {
    const replayed: AgentTaskState = {
      taskId: input.taskId,
      status: "completed",
      revision: state.revision + 1,
      output: { outcome: "persistent_replay" }
    };
    if (projectionEnabled) await projectAgentTaskState(projectionActivityInput(input, binding, replayed, workflow));
    return replayed;
  }
  if (state.status === "created") {
    state = transitionAgentTask(state, { type: "start" });
  }
  while (!isTerminal(state)) {
    if (projectionEnabled && projectedRevision !== state.revision) {
      await projectAgentTaskState(projectionActivityInput(input, binding, state, workflow));
      projectedRevision = state.revision;
    }
    if (state.status === "waiting_input") {
      const requestId = state.pending!.requestId;
      const resumed = await condition(() => state.status !== "waiting_input", waitDuration(state.pending!.expiresAtUnixMs));
      if (!resumed && state.status === "waiting_input") state = transitionAgentTask(state, { type: "expire_wait", requestId });
      continue;
    }
    if (state.status === "waiting_approval") {
      const requestId = state.pending!.requestId;
      const resumed = await condition(() => state.status !== "waiting_approval" || approvalSignal !== undefined, waitDuration(state.pending!.expiresAtUnixMs));
      if (!resumed && state.status === "waiting_approval") {
        state = transitionAgentTask(state, { type: "expire_wait", requestId });
        continue;
      }
      if (state.status !== "waiting_approval") continue;
      const signal = approvalSignal!;
      await resolveAgentTaskApproval({
        taskId: input.taskId, runId: binding.runId, approvalId: signal.approvalId,
        decision: signal.decision, actorUserId: signal.actorUserId,
        ...(input.admission?.requestId === undefined ? {} : { requestId: input.admission.requestId }),
        ...(input.admission?.traceId === undefined ? {} : { traceId: input.admission.traceId })
      });
      approvalSignal = undefined;
      state = transitionAgentTask(state, { type: "resolve_approval", requestId: signal.requestId, decision: signal.decision });
      continue;
    }
    if (step >= maxSteps) {
      state = transitionAgentTask(state, {
        type: "fail", message: `Agent Task exceeded the ${maxSteps} Activity step limit`
      });
      break;
    }

    const resume = state.resume;
    let directive: AgentTaskDirective;
    try {
      if (mcpExecution === undefined) {
        directive = await executeAgentTaskStep({
          taskId: input.taskId,
          runId: binding.runId,
          goal: input.goal,
          ...(input.admission === undefined ? {} : { admission: input.admission }),
          ...(input.shadowEvent === undefined ? {} : { shadowEvent: input.shadowEvent }),
          step,
          ...(checkpoint === undefined ? {} : { checkpoint }),
          ...(resume === undefined ? {} : { resume })
        });
      } else if (checkpoint === undefined) {
        const admission = input.admission;
        if (admission === undefined) throw new Error("External MCP Agent Task admission is unavailable");
        directive = await executeMcpDispatch(createTemporalMcpBeginActivityInput(mcpExecution, {
          taskId: input.taskId,
          runId: binding.runId,
          principalUserId: admission.principalUserId,
          ...(admission.requestId === undefined ? {} : { requestId: admission.requestId }),
          ...(admission.traceId === undefined ? {} : { traceId: admission.traceId })
        }));
      } else {
        directive = await executeMcpDispatch(createTemporalMcpResumeActivityInput(checkpoint, resume));
      }
    } catch (error) {
      state = transitionAgentTask(state, { type: "fail", message: activityFailureMessage(error) });
      break;
    }

    if (isTerminal(state)) {
      break;
    }
    step += 1;
    checkpoint = "checkpoint" in directive ? directive.checkpoint : undefined;
    if (directive.kind === "wait_approval") {
      await requestAgentTaskApproval({
        taskId: input.taskId, runId: binding.runId, approval: directive.approval,
        ...(input.admission?.requestId === undefined ? {} : { requestId: input.admission.requestId }),
        ...(input.admission?.traceId === undefined ? {} : { traceId: input.admission.traceId })
      });
    }
    state = applyDirective(state, directive, promotionReceipt);
  }
  if (projectionEnabled && projectedRevision !== state.revision) {
    await projectAgentTaskState(projectionActivityInput(input, binding, state, workflow));
  }
  await finishAgentTask(terminalActivityInput(input, binding, state));
  return state;
}

function projectionActivityInput(
  input: AgentTaskWorkflowHistoryInput,
  binding: AgentTaskRunBinding,
  state: AgentTaskState,
  workflow: { workflowId: string; runId: string }
): AgentTaskProjectionInput {
  return {
    taskId: input.taskId,
    runId: binding.runId,
    workflowId: workflow.workflowId,
    workflowRunId: workflow.runId,
    workflowStatus: state.status,
    workflowRevision: state.revision,
    ...(input.admission?.requestId === undefined ? {} : { requestId: input.admission.requestId }),
    ...(input.admission?.traceId === undefined ? {} : { traceId: input.admission.traceId })
  };
}

function applyDirective(state: AgentTaskState, directive: AgentTaskDirective, promotionReceipt?: AgentMemoryPromotionReceipt): AgentTaskState {
  switch (directive.kind) {
    case "continue":
      return state.resume === undefined
        ? state
        : { taskId: state.taskId, status: state.status, revision: state.revision };
    case "wait_input":
      return transitionAgentTask(state, {
        type: "request_input", requestId: directive.requestId, prompt: directive.prompt, form: directive.form,
        ...(directive.source === undefined ? {} : { source: directive.source }),
        expiresAtUnixMs: directive.expiresAtUnixMs
      });
    case "wait_approval":
      return transitionAgentTask(state, {
        type: "request_approval", requestId: directive.requestId, approvalId: directive.approval.approvalId, summary: directive.summary,
        expiresAtUnixMs: directive.approval.expiresAtUnixMs
      });
    case "complete":
      return transitionAgentTask(state, {
        type: "complete",
        output: promotionReceipt === undefined ? directive.output : { result: directive.output, promotionReceipt }
      });
    case "failed":
      return transitionAgentTask(state, { type: "fail", message: directive.message });
  }
}

function waitDuration(expiresAtUnixMs: number): number {
  return Math.max(1, expiresAtUnixMs - Date.now());
}

function isTerminal(state: AgentTaskState): boolean {
  return state.status === "completed" || state.status === "failed" || state.status === "cancelled";
}

function activityFailureMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function validMaxSteps(value: number | undefined): number {
  const maxSteps = value ?? 32;
  if (!Number.isInteger(maxSteps) || maxSteps < 1 || maxSteps > 256) {
    throw new Error("Agent Task maxSteps must be an integer between 1 and 256");
  }
  return maxSteps;
}

function assertRunBinding(taskId: string, binding: AgentTaskRunBinding): void {
  if (binding.taskId !== taskId || binding.runId.trim().length === 0) {
    throw new Error("Agent Task admission returned an invalid Task/Run binding");
  }
}

function terminalActivityInput(
  input: AgentTaskWorkflowHistoryInput,
  binding: AgentTaskRunBinding,
  state: AgentTaskState
): AgentTaskFinishInput {
  if (!isTerminal(state)) {
    throw new Error("Agent Task must be terminal before finishing its persistent Run");
  }
  const lastError = state.status === "failed"
    ? boundedTerminalEvidence(state.failure?.message || "Agent Task failed")
    : state.status === "cancelled"
      ? boundedTerminalEvidence(state.cancellation?.reason ?? "")
      : "";
  return {
    taskId: input.taskId,
    runId: binding.runId,
    runStatus: terminalRunStatus(state),
    lastError,
    ...(input.admission?.requestId === undefined ? {} : { requestId: input.admission.requestId }),
    ...(input.admission?.traceId === undefined ? {} : { traceId: input.admission.traceId })
  };
}

function terminalRunStatus(state: AgentTaskState): AgentTaskFinishInput["runStatus"] {
  switch (state.status) {
    case "completed":
    case "failed":
    case "cancelled":
      return state.status;
    default:
      throw new Error(`Agent Task cannot finish its persistent Run from ${state.status}`);
  }
}

function boundedTerminalEvidence(value: string): string {
  return value.trim().slice(0, 256);
}
