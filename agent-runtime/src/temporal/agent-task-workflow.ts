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
import type { AgentTaskActivities, AgentTaskDirective } from "./agent-task-activities.js";
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

  state = transitionAgentTask(state, { type: "start" });
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
