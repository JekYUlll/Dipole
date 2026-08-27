import type { AgentEvent, AgentIdentity, ShadowTaskDispatcher } from "../events/shadow-processor.js";
import type { AgentTaskWorkflowControlPort } from "../control/agent-task-control.js";
import type { AgentTaskState } from "../task/agent-task-state.js";
import type { Connection } from "@temporalio/client";

export interface AgentTaskWorkflowInput {
  taskId: string;
  goal: string;
  maxSteps?: number;
  admission?: AgentTaskAdmissionInput;
  shadowEvent?: {
    eventId: string;
    eventType: string;
    aggregateId: string;
    occurredAt: string;
    payload: Readonly<Record<string, unknown>>;
  };
}

export interface AgentTaskAdmissionInput {
  tenantId: string;
  principalUserId: string;
  agentId: string;
  triggerType: string;
  triggerRef: string;
  eventId: string;
  requestId?: string;
  traceId?: string;
}

export interface TemporalWorkflowHandle {
  workflowId: string;
  runId?: string;
}

interface TemporalWorkflowStartHandle {
  workflowId: string;
  firstExecutionRunId?: string;
  runId?: string;
}

interface TemporalWorkflowStartPort {
  start(workflowType: string, options: {
    taskQueue: string;
    workflowId: string;
    workflowIdConflictPolicy: "USE_EXISTING";
    workflowIdReusePolicy: "REJECT_DUPLICATE";
    args: [AgentTaskWorkflowInput];
  }): Promise<TemporalWorkflowStartHandle>;
}

interface TemporalWorkflowControlHandle {
  query(queryName: string): Promise<unknown>;
  signal(signalName: string, payload: unknown): Promise<void>;
}

interface TemporalWorkflowControlClientPort {
  getHandle(workflowId: string): TemporalWorkflowControlHandle;
}

export class TemporalTaskClient {
  constructor(
    private readonly workflow: TemporalWorkflowStartPort,
    private readonly taskQueue: string
  ) {}

  async start(input: AgentTaskWorkflowInput): Promise<TemporalWorkflowHandle> {
    const handle = await this.workflow.start("agentTaskWorkflow", {
      taskQueue: this.taskQueue,
      workflowId: agentTaskWorkflowId(input.taskId),
      workflowIdConflictPolicy: "USE_EXISTING",
      workflowIdReusePolicy: "REJECT_DUPLICATE",
      args: [input]
    });
    const runId = handle.firstExecutionRunId ?? handle.runId;
    return { workflowId: handle.workflowId, ...(runId === undefined ? {} : { runId }) };
  }
}

export class TemporalShadowTaskDispatcher implements ShadowTaskDispatcher {
  constructor(private readonly tasks: Pick<TemporalTaskClient, "start">) {}

  async dispatch(event: AgentEvent, identity: AgentIdentity, taskId: string): Promise<void> {
    await this.tasks.start({
      taskId,
      goal: `observe ${event.eventType} for ${event.aggregateId}`,
      shadowEvent: event,
      admission: {
        tenantId: identity.tenantId,
        principalUserId: identity.principalUuid,
        agentId: identity.agentUuid,
        triggerType: event.eventType,
        triggerRef: event.aggregateId,
        eventId: event.eventId,
        ...(identity.requestId === undefined ? {} : { requestId: identity.requestId }),
        ...(identity.traceId === undefined ? {} : { traceId: identity.traceId })
      }
    });
  }
}

export class TemporalTaskControlClient implements AgentTaskWorkflowControlPort {
  constructor(private readonly workflow: TemporalWorkflowControlClientPort) {}

  async query(taskId: string): Promise<AgentTaskState> {
    return this.handle(taskId).query("taskState") as Promise<AgentTaskState>;
  }

  async cancel(taskId: string, reason: string): Promise<void> {
    await this.handle(taskId).signal("cancelTask", { reason });
  }

  async resolveApproval(taskId: string, signal: { requestId: string; approvalId: string; decision: "approved" | "denied"; actorUserId: string }): Promise<void> {
    await this.handle(taskId).signal("resolveTaskApproval", signal);
  }

  private handle(taskId: string): TemporalWorkflowControlHandle {
    return this.workflow.getHandle(agentTaskWorkflowId(taskId));
  }
}

export interface TemporalTaskDispatchRuntime extends ShadowTaskDispatcher, AgentTaskWorkflowControlPort {
  start(): Promise<void>;
  stop(): Promise<void>;
}

export function createTemporalTaskDispatchRuntime(config: {
  address: string;
  namespace: string;
  taskQueue: string;
}): TemporalTaskDispatchRuntime {
  let connection: Connection | undefined;
  let dispatcher: TemporalShadowTaskDispatcher | undefined;
  let controls: TemporalTaskControlClient | undefined;
  return {
    async start() {
      if (connection !== undefined) {
        throw new Error("Temporal Task dispatcher is already started");
      }
      const { Client, Connection } = await import("@temporalio/client");
      connection = await Connection.connect({ address: config.address });
      const client = new Client({ connection, namespace: config.namespace });
      dispatcher = new TemporalShadowTaskDispatcher(new TemporalTaskClient(client.workflow, config.taskQueue));
      controls = new TemporalTaskControlClient(client.workflow);
    },
    async dispatch(event, identity, taskId) {
      if (dispatcher === undefined) {
        throw new Error("Temporal Task dispatcher is not started");
      }
      await dispatcher.dispatch(event, identity, taskId);
    },
    async query(taskId) {
      if (controls === undefined) throw new Error("Temporal Task controls are not started");
      return controls.query(taskId);
    },
    async cancel(taskId, reason) {
      if (controls === undefined) throw new Error("Temporal Task controls are not started");
      await controls.cancel(taskId, reason);
    },
    async resolveApproval(taskId, signal) {
      if (controls === undefined) throw new Error("Temporal Task controls are not started");
      await controls.resolveApproval(taskId, signal);
    },
    async stop() {
      dispatcher = undefined;
      controls = undefined;
      try {
        await connection?.close();
      } finally {
        connection = undefined;
      }
    }
  };
}

export function agentTaskWorkflowId(taskId: string): string {
  const normalized = taskId.trim();
  if (normalized.length === 0) {
    throw new Error("Agent Task ID is required for a Temporal Workflow");
  }
  return `dipole-agent-task/${normalized}`;
}
