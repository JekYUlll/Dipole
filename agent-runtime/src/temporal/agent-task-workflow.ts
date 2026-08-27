import {
  condition,
  defineQuery,
  defineSignal,
  proxyActivities,
  setHandler
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
  AgentTaskRunBinding
} from "./agent-task-activities.js";
import type { AgentTaskWorkflowInput } from "./temporal-task-client.js";

export const provideTaskInputSignal = defineSignal<[
  { requestId: string; value: unknown }
]>("provideTaskInput");
export const resolveTaskApprovalSignal = defineSignal<[
  { requestId: string; decision: "approved" | "denied" }
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
const { admitAgentTask, finishAgentTask } = proxyActivities<AgentTaskLifecycleActivities>({
  startToCloseTimeout: "30 seconds",
  retry: {
    initialInterval: "1 second",
    backoffCoefficient: 2,
    maximumInterval: "30 seconds",
    maximumAttempts: 3
  }
});

export async function agentTaskWorkflow(input: AgentTaskWorkflowInput): Promise<AgentTaskState> {
  let state = createAgentTaskState(input.taskId);
  let checkpoint: unknown;
  let step = 0;
  const maxSteps = validMaxSteps(input.maxSteps);

  setHandler(taskStateQuery, () => state);
  setHandler(provideTaskInputSignal, (signal) => {
    if (state.status !== "waiting_input" || state.pending?.kind !== "input" || state.pending.requestId !== signal.requestId) {
      return;
    }
    state = transitionAgentTask(state, { type: "provide_input", requestId: signal.requestId, value: signal.value });
  });
  setHandler(resolveTaskApprovalSignal, (signal) => {
    if (state.status !== "waiting_approval" || state.pending?.kind !== "approval" || state.pending.requestId !== signal.requestId) {
      return;
    }
    state = transitionAgentTask(state, {
      type: "resolve_approval", requestId: signal.requestId, decision: signal.decision
    });
  });
  setHandler(cancelTaskSignal, (signal) => {
    if (!isTerminal(state)) {
      state = transitionAgentTask(state, { type: "cancel", reason: signal.reason });
    }
  });

  const binding = await admitAgentTask(input);
  assertRunBinding(input.taskId, binding);
  if (binding.runStatus === "completed") {
    return {
      taskId: input.taskId,
      status: "completed",
      revision: state.revision + 1,
      output: { outcome: "persistent_replay" }
    };
  }
  if (state.status === "created") {
    state = transitionAgentTask(state, { type: "start" });
  }
  while (!isTerminal(state)) {
    if (state.status === "waiting_input" || state.status === "waiting_approval") {
      await condition(() => state.status !== "waiting_input" && state.status !== "waiting_approval");
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
      directive = await executeAgentTaskStep({
        taskId: input.taskId,
        goal: input.goal,
        step,
        ...(checkpoint === undefined ? {} : { checkpoint }),
        ...(resume === undefined ? {} : { resume })
      });
    } catch (error) {
      state = transitionAgentTask(state, { type: "fail", message: activityFailureMessage(error) });
      break;
    }

    if (isTerminal(state)) {
      break;
    }
    step += 1;
    checkpoint = "checkpoint" in directive ? directive.checkpoint : undefined;
    state = applyDirective(state, directive);
  }
  await finishAgentTask(terminalActivityInput(input, binding, state));
  return state;
}

function applyDirective(state: AgentTaskState, directive: AgentTaskDirective): AgentTaskState {
  switch (directive.kind) {
    case "continue":
      return state.resume === undefined
        ? state
        : { taskId: state.taskId, status: state.status, revision: state.revision };
    case "wait_input":
      return transitionAgentTask(state, {
        type: "request_input", requestId: directive.requestId, prompt: directive.prompt
      });
    case "wait_approval":
      return transitionAgentTask(state, {
        type: "request_approval", requestId: directive.requestId, summary: directive.summary
      });
    case "complete":
      return transitionAgentTask(state, { type: "complete", output: directive.output });
    case "failed":
      return transitionAgentTask(state, { type: "fail", message: directive.message });
  }
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
  input: AgentTaskWorkflowInput,
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
