import * as grpc from "@grpc/grpc-js";

import type { AgentEvent, AgentIdentity } from "../events/shadow-processor.js";
import type { IAgentCapabilityServiceClient } from "../generated/dipole/agent/v1/agent.grpc-client.js";
import type { ConversationSnapshot } from "../generated/dipole/agent/v1/agent.js";
import type { ExecutionContext } from "../runtime/execution-context.js";

const callerService = "dipole-agent";

export interface AgentRunIdentity {
  readonly taskId: string;
  readonly runId: string;
  readonly runStatus: "running" | "completed";
}

export type AgentRunTerminalStatus = "completed" | "failed" | "cancelled";

export interface AgentRunAdmissionRequest {
  readonly tenantId: string;
  readonly principalUserId: string;
  readonly agentId: string;
  readonly triggerType: string;
  readonly triggerRef: string;
  readonly eventId: string;
  readonly requestId?: string;
  readonly traceId?: string;
}

export interface ConversationListItem {
  readonly conversationKey: string;
  readonly targetId: string;
  readonly targetType: number;
  readonly lastMessageId: string;
  readonly lastMessageSeq: string;
  readonly lastMessagePreview: string;
  readonly lastMessageAtUnixMs: string;
  readonly readSeq: string;
  readonly unreadCount: number;
}

export class AgentCapabilityRPCClient {
  constructor(
    private readonly rpc: IAgentCapabilityServiceClient,
    private readonly secret: string,
    private readonly timeoutMs = 2_000
  ) {
    if (!secret.trim()) {
      throw new Error("Agent Capability RPC secret is required");
    }
  }

  async admit(event: AgentEvent, identity: AgentIdentity): Promise<AgentRunIdentity> {
    return this.admitRun({
      tenantId: identity.tenantId,
      principalUserId: identity.principalUuid,
      agentId: identity.agentUuid,
      triggerType: event.eventType,
      triggerRef: event.aggregateId,
      eventId: event.eventId,
      ...(identity.requestId === undefined ? {} : { requestId: identity.requestId }),
      ...(identity.traceId === undefined ? {} : { traceId: identity.traceId })
    });
  }

  async admitRun(input: AgentRunAdmissionRequest): Promise<AgentRunIdentity> {
    const metadata = this.metadata(input.requestId, input.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.admitRun({
        context: this.requestContext(input.requestId, input.traceId),
        tenantId: input.tenantId,
        principalUserId: input.principalUserId,
        agentId: input.agentId,
        triggerType: input.triggerType,
        triggerRef: input.triggerRef,
        eventId: input.eventId,
        runtimeId: callerService,
        mode: "shadow"
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) {
          reject(error ?? new Error("Agent Run admission returned no response"));
          return;
        }
        if (response.runStatus !== "running" && response.runStatus !== "completed") {
          reject(new Error(`Agent Run admission returned unsupported status ${response.runStatus}`));
          return;
        }
        resolve({ taskId: response.taskId, runId: response.runId, runStatus: response.runStatus });
      });
    });
  }

  async complete(taskId: string, runId: string, context?: Pick<ExecutionContext, "requestId" | "traceId">): Promise<void> {
    const metadata = this.metadata(context?.requestId, context?.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.completeRun({
        context: this.requestContext(context?.requestId, context?.traceId), taskId, runId
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) {
          reject(error ?? new Error("Agent Run completion returned no response"));
          return;
        }
        if (response.runStatus !== "completed") {
          reject(new Error(`Agent Run completion returned unsupported status ${response.runStatus}`));
          return;
        }
        resolve();
      });
    });
  }

  async finish(
    taskId: string,
    runId: string,
    runStatus: AgentRunTerminalStatus,
    lastError: string,
    context?: Pick<ExecutionContext, "requestId" | "traceId">
  ): Promise<void> {
    const metadata = this.metadata(context?.requestId, context?.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.finishRun({
        context: this.requestContext(context?.requestId, context?.traceId),
        taskId,
        runId,
        runStatus,
        lastError
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) {
          reject(error ?? new Error("Agent Run terminal transition returned no response"));
          return;
        }
        if (response.runStatus !== runStatus) {
          reject(new Error(`Agent Run terminal transition returned unsupported status ${response.runStatus}`));
          return;
        }
        resolve();
      });
    });
  }

  async listConversations(context: ExecutionContext, limit: number): Promise<readonly ConversationListItem[]> {
    const metadata = this.metadata(context.requestId, context.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.listConversations({
        context: this.requestContext(context.requestId, context.traceId),
        taskId: context.taskId,
        runId: context.runId,
        limit
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) {
          reject(error ?? new Error("Agent conversation list returned no response"));
          return;
        }
        resolve(response.conversations.map((item: ConversationSnapshot) => ({
          conversationKey: item.conversationKey,
          targetId: item.targetId,
          targetType: item.targetType,
          lastMessageId: item.lastMessageId,
          lastMessageSeq: item.lastMessageSeq.toString(),
          lastMessagePreview: item.lastMessagePreview,
          lastMessageAtUnixMs: item.lastMessageAtUnixMs.toString(),
          readSeq: item.readSeq.toString(),
          unreadCount: item.unreadCount
        })));
      });
    });
  }

  private metadata(requestId?: string, traceId?: string): grpc.Metadata {
    const metadata = new grpc.Metadata();
    metadata.set("x-dipole-caller-service", callerService);
    metadata.set("x-dipole-service-token", this.secret);
    if (requestId !== undefined) metadata.set("x-request-id", requestId);
    if (traceId !== undefined) metadata.set("x-trace-id", traceId);
    return metadata;
  }

  private requestContext(requestId?: string, traceId?: string) {
    return {
      principalUserId: "",
      deviceId: "",
      requestId: requestId ?? "",
      traceId: traceId ?? "",
      callerService
    };
  }
}
