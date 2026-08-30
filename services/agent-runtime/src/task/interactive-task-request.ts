import { createHash } from "node:crypto";

import { z } from "zod";

import { AgentTaskControlError } from "../control/agent-task-control.js";
import { agentTaskId, type AgentEvent, type AgentIdentity } from "../events/shadow-processor.js";

const clientRequestId = z.string().trim().min(1).max(64).regex(/^[A-Za-z0-9._:-]+$/);
const interactiveRequestSchema = z.object({
  clientRequestId,
  goal: z.string().trim().min(1).max(4_000)
}).strict();

const interactiveTriggerType = "agent.interactive.requested";

export interface InteractiveTaskRequestIdentity {
  readonly tenantId: string;
  readonly principalUserId: string;
  readonly agentId: string;
  readonly requestId?: string;
  readonly traceId?: string;
}

export interface InteractiveTaskRequest {
  readonly taskId: string;
  readonly event: AgentEvent;
  readonly identity: AgentIdentity;
}

export interface InteractiveTaskDispatcher {
  dispatch(event: AgentEvent, identity: AgentIdentity, taskId: string): Promise<void>;
}

export interface StartInteractiveTaskInput {
  readonly principalUserId: string;
  readonly requestId?: string;
  readonly traceId?: string;
  readonly body: unknown;
}

export interface StartInteractiveTaskResult {
  readonly taskId: string;
  readonly status: "accepted";
}

export class InteractiveTaskStartService {
  constructor(
    private readonly trustedAgent: Pick<InteractiveTaskRequestIdentity, "tenantId" | "agentId">,
    private readonly dispatcher: InteractiveTaskDispatcher
  ) {}

  async start(input: StartInteractiveTaskInput): Promise<StartInteractiveTaskResult> {
    let request: InteractiveTaskRequest;
    try {
      request = createInteractiveTaskRequest(input.body, {
        ...this.trustedAgent,
        principalUserId: input.principalUserId,
        ...(input.requestId === undefined ? {} : { requestId: input.requestId }),
        ...(input.traceId === undefined ? {} : { traceId: input.traceId })
      });
    } catch (error) {
      throw new AgentTaskControlError("invalid_argument", error instanceof Error ? error.message : "Interactive Agent Task input is invalid");
    }
    await this.dispatcher.dispatch(request.event, request.identity, request.taskId);
    return { taskId: request.taskId, status: "accepted" };
  }
}

// The Gateway supplies only the authenticated principal. Runtime-owned identity
// and deterministic IDs keep client input from changing authority or replay scope.
export function createInteractiveTaskRequest(
  raw: unknown,
  trusted: InteractiveTaskRequestIdentity,
  now: Date = new Date()
): InteractiveTaskRequest {
  const input = interactiveRequestSchema.parse(raw);
  const tenantId = required(trusted.tenantId, "tenant ID");
  const principalUuid = required(trusted.principalUserId, "principal user ID");
  const agentUuid = required(trusted.agentId, "Agent ID");
  if (!Number.isFinite(now.valueOf())) throw new Error("Interactive Agent Task clock is invalid");

  const triggerRef = `interactive:${input.clientRequestId}`;
  const requestId = optional(trusted.requestId);
  const traceId = optional(trusted.traceId);
  const identity: AgentIdentity = {
    tenantId,
    principalUuid,
    agentUuid,
    ...(requestId === undefined ? {} : { requestId }),
    ...(traceId === undefined ? {} : { traceId })
  };
  const taskId = agentTaskId({ tenantId, agentUuid, triggerType: interactiveTriggerType, triggerRef });
  const event: AgentEvent = {
    eventId: `interactive:${digest([tenantId, principalUuid, agentUuid, input.clientRequestId])}`,
    eventType: interactiveTriggerType,
    aggregateId: triggerRef,
    occurredAt: now.toISOString(),
    payload: { content: input.goal, request_kind: "interactive" },
    lineage: { origin: { type: "service", id: "dipole-gateway" } }
  };
  return { taskId, event, identity };
}

function digest(parts: readonly string[]): string {
  return createHash("sha256").update(parts.join("\n"), "utf8").digest("hex").slice(0, 48);
}

function required(value: string, label: string): string {
  const normalized = value.trim();
  if (!normalized || normalized.length > 128) throw new Error(`Interactive Agent Task ${label} is invalid`);
  return normalized;
}

function optional(value: string | undefined): string | undefined {
  const normalized = value?.trim();
  return normalized ? normalized.slice(0, 128) : undefined;
}
