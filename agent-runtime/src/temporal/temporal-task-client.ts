import type { AgentEvent, AgentIdentity, ShadowTaskDispatcher } from "../events/shadow-processor.js";
import type { AgentTaskWorkflowControlPort } from "../control/agent-task-control.js";
import type { AgentTaskState } from "../task/agent-task-state.js";
import type { Connection } from "@temporalio/client";
import type { AgentTaskWorkflowInspector } from "../reconcile/agent-task-projection-reconciler.js";
import {
  TemporalMcpWorkflowExecutionCatalog,
  type TemporalMcpWorkflowExecutionV1
} from "./mcp-workflow-envelope.js";

export interface AgentTaskWorkflowInput {
  taskId: string;
  goal: string;
  maxSteps?: number;
  admission?: AgentTaskAdmissionInput;
  shadowEvent?: AgentEvent;
  execution?: never;
}

export interface ExternalMcpAgentTaskWorkflowInput extends Omit<AgentTaskWorkflowInput, "execution"> {
  admission: AgentTaskAdmissionInput;
  execution: TemporalMcpWorkflowExecutionV1;
}

export type AgentTaskWorkflowHistoryInput = AgentTaskWorkflowInput | ExternalMcpAgentTaskWorkflowInput;

export interface AgentTaskAdmissionInput {
  tenantId: string;
  principalUserId: string;
  agentId: string;
  triggerType: string;
  triggerRef: string;
  eventId: string;
  requestId?: string;
  traceId?: string;
  subscriptionId?: string;
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
    args: [AgentTaskWorkflowHistoryInput];
  }): Promise<TemporalWorkflowStartHandle>;
}

interface TemporalWorkflowControlHandle {
  query(queryName: string): Promise<unknown>;
  signal(signalName: string, payload: unknown): Promise<void>;
  describe?(): Promise<{ workflowId: string; runId: string }>;
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
    return startTaskWorkflow(this.workflow, this.taskQueue, input);
  }
}

export interface TemporalMcpTaskStartInput extends Omit<ExternalMcpAgentTaskWorkflowInput, "execution"> {
  readonly routeId: string;
  readonly arguments: unknown;
}

export class TemporalMcpTaskClient {
  constructor(
    private readonly workflow: TemporalWorkflowStartPort,
    private readonly taskQueue: string,
    private readonly catalog: TemporalMcpWorkflowExecutionCatalog
  ) {}

  async start(input: TemporalMcpTaskStartInput): Promise<TemporalWorkflowHandle> {
    const { routeId, arguments: rawArguments, ...task } = input;
    const historyInput: ExternalMcpAgentTaskWorkflowInput = {
      ...task,
      execution: this.catalog.create(routeId, rawArguments)
    };
    return startTaskWorkflow(this.workflow, this.taskQueue, historyInput);
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
        ...(event.subscriptionId === undefined ? {} : { subscriptionId: event.subscriptionId }),
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

  async provideInput(taskId: string, signal: { requestId: string; value: Readonly<Record<string, string | boolean | readonly string[]>> }): Promise<void> {
    await this.handle(taskId).signal("provideTaskInput", signal);
  }

  private handle(taskId: string): TemporalWorkflowControlHandle {
    return this.workflow.getHandle(agentTaskWorkflowId(taskId));
  }
}

export class TemporalTaskWorkflowInspector implements AgentTaskWorkflowInspector {
  constructor(private readonly workflow: TemporalWorkflowControlClientPort) {}

  async inspect(taskId: string): Promise<{ workflowId: string; workflowRunId: string; state: AgentTaskState }> {
    const handle = this.workflow.getHandle(agentTaskWorkflowId(taskId));
    if (handle.describe === undefined) throw new Error("Temporal Workflow describe is unavailable");
    const [state, description] = await Promise.all([
      handle.query("taskState") as Promise<AgentTaskState>,
      handle.describe()
    ]);
    return { workflowId: description.workflowId, workflowRunId: description.runId, state };
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
    async provideInput(taskId, signal) {
      if (controls === undefined) throw new Error("Temporal Task controls are not started");
      await controls.provideInput(taskId, signal);
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

async function startTaskWorkflow(
  workflow: TemporalWorkflowStartPort,
  taskQueue: string,
  input: AgentTaskWorkflowHistoryInput
): Promise<TemporalWorkflowHandle> {
  const handle = await workflow.start("agentTaskWorkflow", {
    taskQueue,
    workflowId: agentTaskWorkflowId(input.taskId),
    workflowIdConflictPolicy: "USE_EXISTING",
    workflowIdReusePolicy: "REJECT_DUPLICATE",
    args: [input]
  });
  const runId = handle.firstExecutionRunId ?? handle.runId;
  return { workflowId: handle.workflowId, ...(runId === undefined ? {} : { runId }) };
}
