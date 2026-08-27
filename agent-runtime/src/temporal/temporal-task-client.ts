export interface AgentTaskWorkflowInput {
  taskId: string;
  goal: string;
  maxSteps?: number;
}

export interface TemporalWorkflowHandle {
  workflowId: string;
  runId?: string;
}

interface TemporalWorkflowStartPort {
  start(workflowType: string, options: {
    taskQueue: string;
    workflowId: string;
    workflowIdConflictPolicy: "USE_EXISTING";
    workflowIdReusePolicy: "REJECT_DUPLICATE";
    args: [AgentTaskWorkflowInput];
  }): Promise<TemporalWorkflowHandle>;
}

export class TemporalTaskClient {
  constructor(
    private readonly workflow: TemporalWorkflowStartPort,
    private readonly taskQueue: string
  ) {}

  async start(input: AgentTaskWorkflowInput): Promise<TemporalWorkflowHandle> {
    return this.workflow.start("agentTaskWorkflow", {
      taskQueue: this.taskQueue,
      workflowId: agentTaskWorkflowId(input.taskId),
      workflowIdConflictPolicy: "USE_EXISTING",
      workflowIdReusePolicy: "REJECT_DUPLICATE",
      args: [input]
    });
  }
}

export function agentTaskWorkflowId(taskId: string): string {
  const normalized = taskId.trim();
  if (normalized.length === 0) {
    throw new Error("Agent Task ID is required for a Temporal Workflow");
  }
  return `dipole-agent-task/${normalized}`;
}
