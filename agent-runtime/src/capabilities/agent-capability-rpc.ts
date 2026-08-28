import * as grpc from "@grpc/grpc-js";

import type { AgentEvent, AgentIdentity } from "../events/shadow-processor.js";
import type { IAgentCapabilityServiceClient } from "../generated/dipole/agent/v1/agent.grpc-client.js";
import type { AppendAgentTaskTimelineEventResponse, ConversationSnapshot, ListAgentTaskTimelineResponse } from "../generated/dipole/agent/v1/agent.js";
import { executionContextSchema, type ExecutionContext } from "../runtime/execution-context.js";
import type { AgentEventSubscription } from "../events/event-subscription.js";
import { createHash } from "node:crypto";
import { canonicalMcpJSON } from "../mcp/canonical-json.js";
import type { ExternalMcpReadinessEvidence } from "../mcp/external-mcp-readiness-evidence.js";

const callerService = "dipole-agent";
const errorCodePattern = /^[a-z][a-z0-9_]{0,63}$/;
const mcpToolRoundResultLimit = 128 * 1024;
const readinessEvidenceRecordSchemaVersion = "dipole.agent.external-mcp-readiness-evidence-record.v1";

export interface AgentRunIdentity {
  readonly taskId: string;
  readonly runId: string;
  readonly runStatus: "running" | "completed";
}

export type AgentRunTerminalStatus = "completed" | "failed" | "cancelled";

export interface AgentMcpToolCommand {
  readonly invocationId: string;
  readonly tenantId: string;
  readonly principalUserId: string;
  readonly agentId: string;
  readonly taskId: string;
  readonly runId: string;
  readonly profileId: string;
  readonly serverId: string;
  readonly toolName: string;
  readonly capabilityId: string;
  readonly arguments: Readonly<Record<string, unknown>>;
  readonly argumentsSha256: string;
  readonly startedAtUnixMs: number;
  readonly status: "running" | "completed" | "failed";
}

export interface AgentMcpToolCommandBeginResult {
  readonly invocationId: string;
  readonly status: "running" | "completed" | "failed";
}

export interface AgentMcpToolCommandTerminalResult {
  readonly invocationId: string;
  readonly status: "completed" | "failed";
}

export interface AgentMcpToolRoundClaim {
  readonly taskId: string;
  readonly runId: string;
  readonly invocationId: string;
  readonly roundId: string;
  readonly roundNumber: 0 | 1;
  readonly requestSha256: string;
  readonly ownerTokenSha256: string;
}

export type AgentMcpToolRoundClaimResult =
  | { readonly outcome: "claimed" }
  | { readonly outcome: "replay_completed"; readonly result: unknown; readonly resultJSON: string; readonly resultSha256: string }
  | { readonly outcome: "replay_failed"; readonly errorCode: string }
  | { readonly outcome: "ambiguous" };

export type AgentMcpToolRoundFinish = {
  readonly roundId: string;
  readonly ownerTokenSha256: string;
} & (
  | { readonly status: "completed"; readonly resultJSON: string; readonly resultSha256: string }
  | { readonly status: "failed"; readonly errorCode: string }
);

export interface AgentRunAdmissionRequest {
  readonly tenantId: string;
  readonly principalUserId: string;
  readonly agentId: string;
  readonly triggerType: string;
  readonly triggerRef: string;
  readonly eventId: string;
  readonly requestId?: string;
  readonly traceId?: string;
  readonly subscriptionId?: string;
}

export interface AgentApprovalBinding {
  approvalId: string;
  capabilityId: string;
  resourceScope: { resourceType: string; resourceId: string; actions: readonly string[] };
  scopeSha256: string;
  argumentsSha256: string;
  nonceSha256: string;
  expiresAtUnixMs: number;
}

export interface AgentApprovalConsumption {
  readonly approvalId: string;
  readonly capabilityId: string;
  readonly scopeSha256: string;
  readonly argumentsSha256: string;
  readonly nonceSha256: string;
}

export interface AgentApprovalGrant {
  readonly approvalId: string;
  readonly capabilityId: string;
  readonly resourceScope: { readonly resourceType: string; readonly resourceId: string; readonly actions: readonly string[] };
  readonly scopeSha256: string;
  readonly argumentsSha256: string;
  readonly nonceSha256: string;
  readonly expiresAtUnixMs: number;
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

export interface AgentTaskControlAuthorization {
  readonly taskId: string;
  readonly taskStatus: string;
  readonly workflow?: AgentTaskWorkflowProjection;
}

export interface AgentTaskWorkflowProjection {
  readonly taskId: string;
  readonly workflowId: string;
  readonly workflowRunId: string;
  readonly workflowStatus: string;
  readonly workflowRevision: number;
}

export interface AgentTaskWorkflowProjectionSnapshotPage {
  readonly tasks: readonly { taskId: string; workflow?: Omit<AgentTaskWorkflowProjection, "taskId"> }[];
  readonly nextCursor: string;
}

export interface AgentArtifactRecord {
  readonly schemaVersion: "dipole.agent.artifact.v1";
  readonly artifactId: string;
  readonly taskId: string;
  readonly runId: string;
  readonly artifactType: string;
  readonly version: number;
  readonly title: string;
  readonly mediaType: string;
  readonly contentSha256: string;
  readonly sizeBytes: number;
  readonly metadata: Readonly<Record<string, unknown>>;
}

export interface AgentMCPReadinessEvidenceReceipt {
  readonly evidenceId: string;
  readonly profileBindingSha256: string;
  readonly runtimeBindingSha256: string;
  readonly contentSha256: string;
  readonly collectedAt: string;
  readonly expiresAt: string;
  readonly created: boolean;
}

export interface AgentMCPReadinessEvidenceResolution {
  readonly evidenceId: string;
  readonly profileBindingSha256: string;
  readonly runtimeBindingSha256: string;
  readonly contentSha256: string;
  readonly collectedAt: string;
  readonly expiresAt: string;
}

export interface AgentArtifactCreateInput {
  readonly tenantId: string;
  readonly taskId: string;
  readonly runId: string;
  readonly artifactType: string;
  readonly version: number;
  readonly title: string;
  readonly mediaType: string;
  readonly content: Uint8Array;
  readonly metadata: Readonly<Record<string, unknown>>;
  readonly requestId?: string;
  readonly traceId?: string;
}

export interface AgentContextMemory {
  readonly memoryId: string;
  readonly memoryType: "working" | "episodic" | "semantic" | "procedural" | "observational";
  readonly content: string;
  readonly compactContent?: string;
  readonly priority: number;
  readonly provenance: { readonly sourceType: string; readonly sourceId: string; readonly uri?: string; readonly sequence?: string };
}

export interface AgentToolInvocationBegin {
  readonly invocationId: string;
  readonly taskId: string;
  readonly runId: string;
  readonly toolName: string;
  readonly capabilityId: string;
  readonly argumentsSha256: string;
  readonly profileId?: string;
  readonly serverId?: string;
  readonly argumentsJson?: string;
  readonly requestId?: string;
  readonly traceId?: string;
  readonly approvalId?: string;
}

export interface AgentToolActionReference {
  readonly resourceType: "message";
  readonly resourceId: string;
  readonly commandKind: "assistant_reply" | "system_message";
  readonly commandId: string;
}

export interface AgentMessageCommandExecutionInput {
  readonly taskId: string;
  readonly runId: string;
  readonly invocationId: string;
  readonly commandKind: "assistant_reply" | "system_message";
  readonly content: string;
  readonly requestId?: string;
  readonly traceId?: string;
}

export type AgentToolInvocationFinish = {
  readonly invocationId: string;
  readonly taskId: string;
  readonly runId: string;
  readonly latencyMs: number;
} & ({ readonly status: "completed"; readonly resultSha256: string; readonly resultBytes: number; readonly actionReference?: AgentToolActionReference } | { readonly status: "failed"; readonly errorCode: string });

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
      ...(event.subscriptionId === undefined ? {} : { subscriptionId: event.subscriptionId }),
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
        mode: "shadow",
        subscriptionId: input.subscriptionId ?? ""
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

  async matchEventSubscriptions(event: AgentEvent, identity: AgentIdentity): Promise<AgentEventSubscription[]> {
    const resourceId = typeof event.payload.conversation_key === "string" ? event.payload.conversation_key.trim() : "";
    if (!resourceId) throw new Error("Agent event has no subscription resource identity");
    const metadata = this.metadata(identity.requestId, identity.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.matchEventSubscriptions({
        context: this.requestContext(identity.requestId, identity.traceId),
        tenantId: identity.tenantId,
        agentId: identity.agentUuid,
        eventType: event.eventType,
        resourceType: "conversation",
        resourceId
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) return reject(error ?? new Error("Agent Event Subscription lookup returned no response"));
        try {
          resolve(response.subscriptions.map((item) => ({
            subscriptionId: item.subscriptionId,
            definitionId: item.definitionId,
            definitionVersion: Number(item.definitionVersion),
            tenantId: item.tenantId,
            agentId: item.agentId,
            eventType: item.eventType,
            resourceType: item.resourceType,
            resourceId: item.resourceId,
            filterKind: item.filterKind,
            filter: JSON.parse(new TextDecoder().decode(item.filterJson)) as unknown
          })) as AgentEventSubscription[]);
        } catch (decodeError) {
          reject(decodeError);
        }
      });
    });
  }

  async listContextMemories(context: Pick<ExecutionContext, "taskId" | "runId" | "requestId" | "traceId">, resourceType: string, resourceId: string, limit = 20): Promise<AgentContextMemory[]> {
    const metadata = this.metadata(context.requestId, context.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.listContextMemories({
        context: this.requestContext(context.requestId, context.traceId), taskId: context.taskId, runId: context.runId,
        resourceType, resourceId, limit
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) return reject(error ?? new Error("Agent Memory lookup returned no response"));
        try {
          resolve(response.memories.map((item) => {
            if (item.provenance === undefined) throw new Error(`Agent Memory ${item.memoryId} has no provenance`);
            if (!["working", "episodic", "semantic", "procedural", "observational"].includes(item.memoryType)) {
              throw new Error(`Agent Memory ${item.memoryId} has unsupported type ${item.memoryType}`);
            }
            return {
              memoryId: item.memoryId,
              memoryType: item.memoryType as AgentContextMemory["memoryType"],
              content: item.content,
              ...(item.compactContent ? { compactContent: item.compactContent } : {}),
              priority: item.priority,
              provenance: {
                sourceType: item.provenance.sourceType, sourceId: item.provenance.sourceId,
                ...(item.provenance.uri ? { uri: item.provenance.uri } : {}),
                ...(item.provenance.sequence ? { sequence: item.provenance.sequence } : {})
              }
            };
          }));
        } catch (decodeError) { reject(decodeError); }
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

  async requestApproval(taskId: string, runId: string, approval: AgentApprovalBinding, context?: Pick<ExecutionContext, "requestId" | "traceId">): Promise<void> {
    const metadata = this.metadata(context?.requestId, context?.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.requestApproval({
        context: this.requestContext(context?.requestId, context?.traceId), taskId, runId,
        approvalId: approval.approvalId, capabilityId: approval.capabilityId,
        resourceScope: { ...approval.resourceScope, actions: [...approval.resourceScope.actions] },
        scopeSha256: approval.scopeSha256, argumentsSha256: approval.argumentsSha256,
        nonceSha256: approval.nonceSha256, expiresAtUnixMs: BigInt(approval.expiresAtUnixMs)
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) return reject(error ?? new Error("Agent Approval request returned no response"));
        if (response.approvalId !== approval.approvalId || (response.status !== "pending" && response.status !== "approved")) return reject(new Error("Agent Approval request returned a conflicting binding"));
        resolve();
      });
    });
  }

  async resolveApproval(taskId: string, runId: string, approvalId: string, decision: "approved" | "denied", actorUserId: string, context?: Pick<ExecutionContext, "requestId" | "traceId">): Promise<void> {
    const metadata = this.metadata(context?.requestId, context?.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.resolveApproval({
        context: this.requestContext(context?.requestId, context?.traceId), taskId, runId, approvalId, actorUserId, decision
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) return reject(error ?? new Error("Agent Approval resolution returned no response"));
        const expected = decision === "approved" ? "approved" : "revoked";
        if (response.approvalId !== approvalId || response.status !== expected) return reject(new Error("Agent Approval resolution returned a conflicting state"));
        resolve();
      });
    });
  }

  async consumeApproval(
    taskId: string,
    runId: string,
    consumption: AgentApprovalConsumption,
    context?: Pick<ExecutionContext, "requestId" | "traceId">
  ): Promise<void> {
    const metadata = this.metadata(context?.requestId, context?.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.consumeApproval({
        context: this.requestContext(context?.requestId, context?.traceId), taskId, runId,
        approvalId: consumption.approvalId, capabilityId: consumption.capabilityId,
        scopeSha256: consumption.scopeSha256, argumentsSha256: consumption.argumentsSha256,
        nonceSha256: consumption.nonceSha256, mode: "active"
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) return reject(error ?? new Error("Agent Approval consumption returned no response"));
        if (response.approvalId !== consumption.approvalId || response.status !== "consumed") {
          return reject(new Error("Agent Approval consumption returned a conflicting state"));
        }
        resolve();
      });
    });
  }

  async resolveApprovalGrant(
    taskId: string,
    runId: string,
    capabilityId: string,
    resourceScope: { readonly resourceType: string; readonly resourceId: string; readonly actions: readonly string[] },
    argumentsSha256: string,
    context?: { requestId?: string; traceId?: string }
  ): Promise<AgentApprovalGrant> {
    if (!taskId.trim() || !runId.trim() || !capabilityId.trim() || !validSHA256(argumentsSha256)) {
      throw new Error("Agent Approval grant request is invalid");
    }
    const expectedScopeSha256 = agentResourceScopeSHA256(resourceScope);
    const metadata = this.metadata(context?.requestId, context?.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.resolveApprovalGrant({
        context: this.requestContext(context?.requestId, context?.traceId), taskId, runId, capabilityId,
        resourceScope: { resourceType: resourceScope.resourceType, resourceId: resourceScope.resourceId, actions: [...resourceScope.actions] },
        argumentsSha256
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response?.resourceScope === undefined) {
          reject(error ?? new Error("Agent Approval grant returned no binding"));
          return;
        }
        let expiresAtUnixMs: number;
        try {
          expiresAtUnixMs = safeUnixMilliseconds(response.expiresAtUnixMs);
        } catch (validationError) {
          reject(validationError);
          return;
        }
        if (!validBoundedIdentifier(response.approvalId, 128) || response.capabilityId !== capabilityId || !sameAgentResourceScope(response.resourceScope, resourceScope) ||
            response.scopeSha256 !== expectedScopeSha256 || response.argumentsSha256 !== argumentsSha256 || !validSHA256(response.nonceSha256) ||
            expiresAtUnixMs <= Date.now()) {
          reject(new Error("Agent Approval grant returned conflicting evidence"));
          return;
        }
        resolve({
          approvalId: response.approvalId, capabilityId, resourceScope: {
            resourceType: response.resourceScope.resourceType, resourceId: response.resourceScope.resourceId,
            actions: [...response.resourceScope.actions]
          }, scopeSha256: response.scopeSha256, argumentsSha256, nonceSha256: response.nonceSha256, expiresAtUnixMs
        });
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

  async authorizeTaskControl(taskId: string, principalUserId: string, context?: { requestId?: string; traceId?: string }): Promise<AgentTaskControlAuthorization> {
    const metadata = this.metadata(context?.requestId, context?.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.authorizeTaskControl({
        context: this.requestContext(context?.requestId, context?.traceId), taskId, principalUserId
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) {
          reject(error ?? new Error("Agent Task control authorization returned no response"));
          return;
        }
        if (response.taskId !== taskId || response.taskStatus.trim().length === 0) {
          reject(new Error("Agent Task control authorization returned a conflicting binding"));
          return;
        }
        const workflow = response.workflowId.trim().length === 0 ? undefined : {
          taskId: response.taskId,
          workflowId: response.workflowId,
          workflowRunId: response.workflowRunId,
          workflowStatus: response.workflowStatus,
          workflowRevision: safeRevision(response.workflowRevision)
        };
        resolve({ taskId: response.taskId, taskStatus: response.taskStatus, ...(workflow === undefined ? {} : { workflow }) });
      });
    });
  }

  async listAgentTaskTimeline(taskId: string, principalUserId: string, afterSeq: bigint, limit: number, context?: { requestId?: string; traceId?: string }): Promise<import("../control/agent-task-control.js").AgentTaskTimeline> {
    const metadata = this.metadata(context?.requestId, context?.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.listAgentTaskTimeline({
        context: this.requestContext(context?.requestId, context?.traceId), taskId, principalUserId, afterSeq, limit
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response: ListAgentTaskTimelineResponse | undefined) => {
        if (error !== null || response === undefined) {
          reject(error ?? new Error("Agent Task Timeline returned no response"));
          return;
        }
        if (response.taskId !== taskId || response.schemaVersion.trim().length === 0 || response.nextCursor.trim().length > 384) {
          reject(new Error("Agent Task Timeline returned a conflicting binding"));
          return;
        }
        resolve({
          schemaVersion: response.schemaVersion, taskId: response.taskId, revision: response.revision,
          events: response.events.map(event => ({
            eventSeq: event.eventSeq, eventId: event.eventId, runId: event.runId, kind: event.kind,
            status: event.status, capabilityId: event.capabilityId, approvalId: event.approvalId,
            occurredAtUnixMs: event.occurredAtUnixMs
          })),
          nextCursor: response.nextCursor
        });
      });
    });
  }

  async appendAgentTaskTimelineEvent(input: {
    readonly eventId: string; readonly taskId: string; readonly runId: string; readonly kind: string; readonly status: string;
    readonly capabilityId?: string; readonly approvalId?: string; readonly occurredAtUnixMs: number;
    readonly requestId?: string; readonly traceId?: string;
  }): Promise<{ readonly eventSeq: bigint; readonly eventId: string }> {
    const metadata = this.metadata(input.requestId, input.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.appendAgentTaskTimelineEvent({
        context: this.requestContext(input.requestId, input.traceId), eventId: input.eventId, taskId: input.taskId, runId: input.runId,
        kind: input.kind, status: input.status, capabilityId: input.capabilityId ?? "", approvalId: input.approvalId ?? "",
        occurredAtUnixMs: BigInt(input.occurredAtUnixMs)
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response: AppendAgentTaskTimelineEventResponse | undefined) => {
        if (error !== null || response === undefined) {
          reject(error ?? new Error("Agent Task Timeline append returned no response"));
          return;
        }
        if (response.eventId !== input.eventId || response.eventSeq <= 0n) {
          reject(new Error("Agent Task Timeline append returned a conflicting binding"));
          return;
        }
        resolve({ eventSeq: response.eventSeq, eventId: response.eventId });
      });
    });
  }

  async resolveMcpContext(taskId: string, runId: string, principalUserId: string, context?: { requestId?: string; traceId?: string }): Promise<ExecutionContext> {
    const metadata = this.metadata(context?.requestId, context?.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.resolveMcpContext({
        context: this.requestContext(context?.requestId, context?.traceId), taskId, runId, principalUserId
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) {
          reject(error ?? new Error("Agent MCP context resolution returned no response"));
          return;
        }
        try {
          if (response.runtimeId !== callerService || (response.mode !== "shadow" && response.mode !== "active")) {
            throw new Error("Agent MCP context returned a conflicting Runtime binding");
          }
          resolve(executionContextSchema.parse({
            tenantId: response.tenantId, principalUuid: response.principalUserId, agentUuid: response.agentId,
            ...(response.delegatedByUserId.trim() === "" ? {} : { delegatedByUuid: response.delegatedByUserId }),
            taskId, runId, mode: response.mode, permissions: response.permissions,
            resourceScopes: response.resourceScopes.map((scope) => ({
              resourceType: scope.resourceType, resourceId: scope.resourceId, actions: scope.actions
            })),
            approvedCapabilities: response.approvedCapabilities,
            ...(context?.requestId === undefined ? {} : { requestId: context.requestId }),
            ...(context?.traceId === undefined ? {} : { traceId: context.traceId })
          }));
        } catch (validationError) {
          reject(validationError);
        }
      });
    });
  }

  async begin(input: AgentToolInvocationBegin): Promise<void> {
    const result = await this.beginMcpToolCommand(input);
    if (result.status !== "running") throw new Error("Agent Tool invocation begin returned conflicting evidence");
  }

  async beginMcpToolCommand(input: AgentToolInvocationBegin): Promise<AgentMcpToolCommandBeginResult> {
    const metadata = this.metadata(input.requestId, input.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.beginMcpToolInvocation({
        context: this.requestContext(input.requestId, input.traceId), taskId: input.taskId, runId: input.runId,
        invocationId: input.invocationId, toolName: input.toolName, capabilityId: input.capabilityId,
        argumentsSha256: input.argumentsSha256, approvalId: input.approvalId ?? "",
        profileId: input.profileId ?? "", serverId: input.serverId ?? "", argumentsJson: Buffer.from(input.argumentsJson ?? "", "utf8")
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) return reject(error ?? new Error("Agent Tool invocation begin returned no response"));
        if (response.invocationId !== input.invocationId || (response.status !== "running" && response.status !== "completed" && response.status !== "failed")) {
          return reject(new Error("Agent Tool invocation begin returned conflicting evidence"));
        }
        resolve({ invocationId: response.invocationId, status: response.status });
      });
    });
  }

  async resolveMcpToolCommand(taskId: string, runId: string, invocationId: string): Promise<AgentMcpToolCommand> {
    const metadata = this.metadata();
    return new Promise((resolve, reject) => {
      this.rpc.resolveMcpToolCommand({
        context: this.requestContext(), taskId, runId, invocationId
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) return reject(error ?? new Error("Agent MCP Tool command returned no response"));
        try {
          const decoded = JSON.parse(Buffer.from(response.argumentsJson).toString("utf8")) as unknown;
          if (typeof decoded !== "object" || decoded === null || Array.isArray(decoded)) throw new Error();
          const canonical = canonicalMcpJSON(decoded);
          const digest = createHash("sha256").update(canonical).digest("hex");
          const startedAtUnixMs = safeUnixMilliseconds(response.startedAtUnixMs);
          if (response.status !== "running" && response.status !== "completed" && response.status !== "failed") throw new Error();
          if (response.taskId !== taskId || response.runId !== runId || response.invocationId !== invocationId ||
              Buffer.from(response.argumentsJson).toString("utf8") !== canonical || response.argumentsSha256 !== digest) {
            throw new Error();
          }
          resolve({
            invocationId, tenantId: response.tenantId, principalUserId: response.principalUserId, agentId: response.agentId,
            taskId, runId, profileId: response.profileId, serverId: response.serverId, toolName: response.toolName,
            capabilityId: response.capabilityId, arguments: decoded as Record<string, unknown>, argumentsSha256: digest, startedAtUnixMs,
            status: response.status
          });
        } catch {
          reject(new Error("Agent MCP Tool command returned conflicting evidence"));
        }
      });
    });
  }

  async claimMcpToolRound(input: AgentMcpToolRoundClaim): Promise<AgentMcpToolRoundClaimResult> {
    assertSha256(input.roundId, "round ID");
    assertSha256(input.requestSha256, "request digest");
    assertSha256(input.ownerTokenSha256, "owner token digest");
    const metadata = this.metadata();
    return new Promise((resolve, reject) => {
      this.rpc.claimMcpToolRound({
        context: this.requestContext(), taskId: input.taskId, runId: input.runId, invocationId: input.invocationId,
        roundId: input.roundId, roundNumber: input.roundNumber, requestSha256: input.requestSha256,
        ownerTokenSha256: input.ownerTokenSha256
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) return reject(error ?? new Error("Agent MCP Tool round claim returned no response"));
        try {
          if (response.roundId !== input.roundId) throw new Error();
          const resultJSON = Buffer.from(response.resultJson).toString("utf8");
          switch (response.outcome) {
            case "claimed":
            case "ambiguous":
              if (resultJSON !== "" || response.resultSha256 !== "" || response.errorCode !== "") throw new Error();
              return resolve({ outcome: response.outcome });
            case "replay_failed":
              if (resultJSON !== "" || response.resultSha256 !== "" || !errorCodePattern.test(response.errorCode)) throw new Error();
              return resolve({ outcome: response.outcome, errorCode: response.errorCode });
            case "replay_completed": {
              const result = JSON.parse(resultJSON) as unknown;
              if (!isRecord(result) || Buffer.byteLength(resultJSON, "utf8") > mcpToolRoundResultLimit) throw new Error();
              const canonical = canonicalMcpJSON(result);
              const digest = createHash("sha256").update(canonical).digest("hex");
              if (canonical !== resultJSON || digest !== response.resultSha256 || response.errorCode !== "") throw new Error();
              return resolve({ outcome: response.outcome, result, resultJSON, resultSha256: digest });
            }
            default:
              throw new Error();
          }
        } catch {
          reject(new Error("Agent MCP Tool round claim returned conflicting evidence"));
        }
      });
    });
  }

  async finishMcpToolRound(input: AgentMcpToolRoundFinish): Promise<void> {
    assertSha256(input.roundId, "round ID");
    assertSha256(input.ownerTokenSha256, "owner token digest");
    if (input.status === "completed") {
      const decoded = JSON.parse(input.resultJSON) as unknown;
      if (!isRecord(decoded) || Buffer.byteLength(input.resultJSON, "utf8") > mcpToolRoundResultLimit) {
        throw new Error("Agent MCP Tool round completion evidence is invalid");
      }
      const canonical = canonicalMcpJSON(decoded);
      if (canonical !== input.resultJSON || createHash("sha256").update(canonical).digest("hex") !== input.resultSha256) {
        throw new Error("Agent MCP Tool round completion evidence is invalid");
      }
    } else if (!errorCodePattern.test(input.errorCode)) {
      throw new Error("Agent MCP Tool round failure evidence is invalid");
    }
    const metadata = this.metadata();
    return new Promise((resolve, reject) => {
      this.rpc.finishMcpToolRound({
        context: this.requestContext(), roundId: input.roundId, ownerTokenSha256: input.ownerTokenSha256,
        status: input.status, resultJson: Buffer.from(input.status === "completed" ? input.resultJSON : "", "utf8"),
        resultSha256: input.status === "completed" ? input.resultSha256 : "", errorCode: input.status === "failed" ? input.errorCode : ""
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) return reject(error ?? new Error("Agent MCP Tool round finish returned no response"));
        if (response.roundId !== input.roundId || response.status !== input.status) return reject(new Error("Agent MCP Tool round finish returned conflicting evidence"));
        resolve();
      });
    });
  }

  async finishMcpToolInvocationFromRound(input: {
    readonly taskId: string;
    readonly runId: string;
    readonly invocationId: string;
    readonly roundId: string;
  }): Promise<AgentMcpToolCommandTerminalResult> {
    assertSha256(input.roundId, "round ID");
    const metadata = this.metadata();
    return new Promise((resolve, reject) => {
      this.rpc.finishMcpToolInvocationFromRound({
        context: this.requestContext(), taskId: input.taskId, runId: input.runId,
        invocationId: input.invocationId, roundId: input.roundId
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) {
          return reject(error ?? new Error("Agent MCP Tool invocation terminal returned no response"));
        }
        if (response.invocationId !== input.invocationId || (response.status !== "completed" && response.status !== "failed")) {
          return reject(new Error("Agent MCP Tool invocation terminal returned conflicting evidence"));
        }
        resolve({ invocationId: response.invocationId, status: response.status });
      });
    });
  }

  async finishToolInvocation(input: AgentToolInvocationFinish): Promise<void> {
    const metadata = this.metadata();
    return new Promise((resolve, reject) => {
      this.rpc.finishMcpToolInvocation({
        context: this.requestContext(), taskId: input.taskId, runId: input.runId, invocationId: input.invocationId,
        status: input.status, resultSha256: input.status === "completed" ? input.resultSha256 : "",
        resultBytes: BigInt(input.status === "completed" ? input.resultBytes : 0), latencyMs: BigInt(input.latencyMs),
        errorCode: input.status === "failed" ? input.errorCode : "",
        ...(input.status === "completed" && input.actionReference !== undefined ? { actionReference: {
          resourceType: input.actionReference.resourceType, resourceId: input.actionReference.resourceId,
          commandKind: input.actionReference.commandKind, commandId: input.actionReference.commandId
        } } : {})
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) return reject(error ?? new Error("Agent Tool invocation finish returned no response"));
        if (response.invocationId !== input.invocationId || response.status !== input.status) return reject(new Error("Agent Tool invocation finish returned conflicting evidence"));
        resolve();
      });
    });
  }

  async executeMessageCommand(input: AgentMessageCommandExecutionInput): Promise<AgentToolActionReference> {
    const content = input.content.trim();
    if (!content || !input.taskId.trim() || !input.runId.trim() || !input.invocationId.trim()) {
      throw new Error("Agent Message Command input is invalid");
    }
    const metadata = this.metadata(input.requestId, input.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.executeMcpMessageCommand({
        context: this.requestContext(input.requestId, input.traceId), taskId: input.taskId, runId: input.runId,
        invocationId: input.invocationId, commandKind: input.commandKind, content
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response?.actionReference === undefined) {
          reject(error ?? new Error("Agent Message Command returned no action reference"));
          return;
        }
        const reference = response.actionReference;
        const commandId = reference.commandId.trim();
        const clientMessageId = createHash("sha256").update(`dipole.agent.command.v1\n${input.commandKind}\n${commandId}`).digest("hex");
        if (reference.resourceType !== "message" || !validBoundedIdentifier(reference.resourceId, 64) ||
            reference.commandKind !== input.commandKind || !validBoundedIdentifier(commandId, 128) || response.clientMessageId !== clientMessageId) {
          reject(new Error("Agent Message Command returned conflicting evidence"));
          return;
        }
        resolve({ resourceType: "message", resourceId: reference.resourceId, commandKind: input.commandKind, commandId });
      });
    });
  }

  async projectTaskWorkflowState(input: AgentTaskWorkflowProjection & { runId: string }, context?: { requestId?: string; traceId?: string }): Promise<AgentTaskWorkflowProjection> {
    const metadata = this.metadata(context?.requestId, context?.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.projectTaskWorkflowState({
        context: this.requestContext(context?.requestId, context?.traceId),
        taskId: input.taskId,
        runId: input.runId,
        workflowId: input.workflowId,
        workflowRunId: input.workflowRunId,
        workflowStatus: input.workflowStatus,
        workflowRevision: BigInt(input.workflowRevision)
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) {
          reject(error ?? new Error("Agent Task Workflow projection returned no response"));
          return;
        }
        const projection = {
          taskId: response.taskId,
          workflowId: response.workflowId,
          workflowRunId: response.workflowRunId,
          workflowStatus: response.workflowStatus,
          workflowRevision: safeRevision(response.workflowRevision)
        };
        if (projection.taskId !== input.taskId || projection.workflowId !== input.workflowId || projection.workflowRunId !== input.workflowRunId ||
            projection.workflowStatus !== input.workflowStatus || projection.workflowRevision !== input.workflowRevision) {
          reject(new Error("Agent Task Workflow projection returned a conflicting binding"));
          return;
        }
        resolve(projection);
      });
    });
  }

  async listTaskWorkflowProjectionSnapshots(afterTaskId: string, pageSize: number): Promise<AgentTaskWorkflowProjectionSnapshotPage> {
    const metadata = this.metadata();
    return new Promise((resolve, reject) => {
      this.rpc.listTaskWorkflowProjectionSnapshots({
        context: this.requestContext(), afterTaskId, pageSize
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) {
          reject(error ?? new Error("Agent Task Workflow projection page returned no response"));
          return;
        }
        resolve({
          tasks: response.tasks.map((task) => ({
            taskId: task.taskId,
            ...(task.hasWorkflow ? { workflow: {
              workflowId: task.workflowId,
              workflowRunId: task.workflowRunId,
              workflowStatus: task.workflowStatus,
              workflowRevision: safeRevision(task.workflowRevision)
            } } : {})
          })),
          nextCursor: response.nextCursor
        });
      });
    });
  }

  async createArtifact(input: AgentArtifactCreateInput): Promise<AgentArtifactRecord> {
    const metadataJSON = Buffer.from(JSON.stringify(input.metadata), "utf8");
    if (input.content.byteLength < 1 || input.content.byteLength > 1_048_576 || metadataJSON.byteLength > 16_384 ||
        input.version < 1 || !Number.isSafeInteger(input.version)) {
      throw new Error("Agent Artifact input exceeds the v1 contract");
    }
    const contentSha256 = createHash("sha256").update(input.content).digest("hex");
    const artifactId = createHash("sha256").update([
      "dipole.agent.artifact.v1", input.taskId.trim(), input.runId.trim(), input.artifactType.trim(),
      input.version.toString(), contentSha256
    ].join("\n"), "utf8").digest("hex");
    const rpcMetadata = this.metadata(input.requestId, input.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.createArtifact({
        context: this.requestContext(input.requestId, input.traceId), tenantId: input.tenantId,
        taskId: input.taskId, runId: input.runId, artifactType: input.artifactType,
        version: input.version, title: input.title, mediaType: input.mediaType,
        content: input.content, metadataJson: metadataJSON
      }, rpcMetadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response?.artifact === undefined) {
          reject(error ?? new Error("Agent Artifact creation returned no record"));
          return;
        }
        const artifact = response.artifact;
        const sizeBytes = Number(artifact.sizeBytes);
        if (artifact.schemaVersion !== "dipole.agent.artifact.v1" || artifact.artifactId !== artifactId || artifact.taskId !== input.taskId ||
            artifact.runId !== input.runId || artifact.artifactType !== input.artifactType || artifact.version !== input.version ||
            artifact.title !== input.title.trim() || artifact.mediaType !== input.mediaType.trim() ||
            artifact.contentSha256 !== contentSha256 || sizeBytes !== input.content.byteLength || !Number.isSafeInteger(sizeBytes)) {
          reject(new Error("Agent Artifact creation returned conflicting evidence"));
          return;
        }
        let parsedMetadata: unknown;
        try {
          parsedMetadata = JSON.parse(Buffer.from(artifact.metadataJson).toString("utf8"));
        } catch {
          reject(new Error("Agent Artifact creation returned invalid metadata"));
          return;
        }
        if (parsedMetadata === null || typeof parsedMetadata !== "object" || Array.isArray(parsedMetadata)) {
          reject(new Error("Agent Artifact creation returned non-object metadata"));
          return;
        }
        if (canonicalJSON(parsedMetadata) !== canonicalJSON(input.metadata)) {
          reject(new Error("Agent Artifact creation returned conflicting metadata"));
          return;
        }
        resolve({
          schemaVersion: artifact.schemaVersion, artifactId: artifact.artifactId, taskId: artifact.taskId,
          runId: artifact.runId, artifactType: artifact.artifactType, version: artifact.version,
          title: artifact.title, mediaType: artifact.mediaType, contentSha256: artifact.contentSha256,
          sizeBytes, metadata: parsedMetadata as Record<string, unknown>
        });
      });
    });
  }

  async publishMcpReadinessEvidence(
    tenantId: string,
    evidence: ExternalMcpReadinessEvidence,
    expiresAt: string,
    context: { readonly requestId?: string; readonly traceId?: string } = {}
  ): Promise<AgentMCPReadinessEvidenceReceipt> {
    if (!validBoundedIdentifier(tenantId, 64)) throw new Error("Agent MCP readiness evidence tenant is invalid");
    const content = canonicalReadinessEvidenceJSON(evidence);
    const completedAt = canonicalISOString(evidence.completedAt);
    const expiry = canonicalISOString(expiresAt);
    const completedAtMs = Date.parse(completedAt);
    const expiresAtMs = Date.parse(expiry);
    if (expiresAtMs <= completedAtMs || expiresAtMs - completedAtMs > 60 * 60 * 1_000) {
      throw new Error("Agent MCP readiness evidence expiry is invalid");
    }
    const contentSha256 = createHash("sha256").update(content, "utf8").digest("hex");
    const requestId = context.requestId ?? "";
    const traceId = context.traceId ?? "";
    const evidenceId = createHash("sha256").update([
      readinessEvidenceRecordSchemaVersion, tenantId, evidence.profileBindingSha256,
      evidence.bindingSha256, contentSha256, callerService, requestId, traceId, expiry
    ].join("\n"), "utf8").digest("hex");
    const metadata = this.metadata(context.requestId, context.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.publishMcpReadinessEvidence({
        context: this.requestContext(context.requestId, context.traceId), tenantId,
        profileBindingSha256: evidence.profileBindingSha256, evidenceJson: Buffer.from(content, "utf8"),
        expiresAtUnixMs: BigInt(expiresAtMs)
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) {
          reject(error ?? new Error("Agent MCP readiness evidence publication returned no receipt"));
          return;
        }
        try {
          const collectedAt = new Date(safeUnixMilliseconds(response.collectedAtUnixMs)).toISOString();
          const returnedExpiry = new Date(safeUnixMilliseconds(response.expiresAtUnixMs)).toISOString();
          if (response.evidenceId !== evidenceId || response.schemaVersion !== readinessEvidenceRecordSchemaVersion ||
              response.profileBindingSha256 !== evidence.profileBindingSha256 || response.runtimeBindingSha256 !== evidence.bindingSha256 ||
              response.contentSha256 !== contentSha256 || response.status !== "recorded" || collectedAt !== completedAt || returnedExpiry !== expiry) {
            throw new Error();
          }
          resolve({ evidenceId, profileBindingSha256: evidence.profileBindingSha256, runtimeBindingSha256: evidence.bindingSha256,
            contentSha256, collectedAt, expiresAt: expiry, created: response.created });
        } catch {
          reject(new Error("Agent MCP readiness evidence publication returned conflicting evidence"));
        }
      });
    });
  }

  async resolveFreshMcpReadinessEvidence(
    tenantId: string,
    profileBindingSha256: string,
    runtimeBindingSha256: string,
    context: { readonly requestId?: string; readonly traceId?: string } = {}
  ): Promise<AgentMCPReadinessEvidenceResolution | undefined> {
    if (!validBoundedIdentifier(tenantId, 64) || !validSHA256(profileBindingSha256) || !validSHA256(runtimeBindingSha256)) {
      throw new Error("Agent MCP readiness evidence lookup is invalid");
    }
    return new Promise((resolve, reject) => {
      this.rpc.resolveFreshMcpReadinessEvidence({
        context: this.requestContext(context.requestId, context.traceId), tenantId,
        profileBindingSha256, runtimeBindingSha256
      }, this.metadata(context.requestId, context.traceId), { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) {
          reject(error ?? new Error("Agent MCP readiness evidence resolution returned no response"));
          return;
        }
        try {
          if (!response.found) {
            if (response.evidenceId !== "" || response.schemaVersion !== "" || response.profileBindingSha256 !== "" ||
                response.runtimeBindingSha256 !== "" || response.contentSha256 !== "" || response.status !== "" ||
                response.collectedAtUnixMs !== 0n || response.expiresAtUnixMs !== 0n) throw new Error();
            resolve(undefined);
            return;
          }
          const collectedAtMs = safeUnixMilliseconds(response.collectedAtUnixMs);
          const expiresAtMs = safeUnixMilliseconds(response.expiresAtUnixMs);
          if (!validSHA256(response.evidenceId) || response.schemaVersion !== readinessEvidenceRecordSchemaVersion ||
              response.profileBindingSha256 !== profileBindingSha256 || response.runtimeBindingSha256 !== runtimeBindingSha256 ||
              !validSHA256(response.contentSha256) || response.status !== "recorded" || collectedAtMs >= expiresAtMs) throw new Error();
          resolve({
            evidenceId: response.evidenceId, profileBindingSha256, runtimeBindingSha256,
            contentSha256: response.contentSha256, collectedAt: new Date(collectedAtMs).toISOString(),
            expiresAt: new Date(expiresAtMs).toISOString()
          });
        } catch {
          reject(new Error("Agent MCP readiness evidence resolution returned conflicting evidence"));
        }
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

function canonicalJSON(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(canonicalJSON).join(",")}]`;
  if (value !== null && typeof value === "object") {
    return `{${Object.entries(value as Record<string, unknown>).sort(([left], [right]) => left.localeCompare(right)).map(([key, item]) => `${JSON.stringify(key)}:${canonicalJSON(item)}`).join(",")}}`;
  }
  return JSON.stringify(value) ?? "null";
}

function canonicalReadinessEvidenceJSON(evidence: ExternalMcpReadinessEvidence): string {
  if (evidence.schemaVersion !== "dipole.agent.external-mcp-readiness-evidence.v2" ||
      !/^[a-f0-9]{64}$/.test(evidence.bindingSha256) || !/^[a-f0-9]{64}$/.test(evidence.profileBindingSha256)) {
    throw new Error("Agent MCP readiness evidence is invalid");
  }
  const startedAt = canonicalISOString(evidence.startedAt);
  const completedAt = canonicalISOString(evidence.completedAt);
  const preflightCheckedAt = canonicalISOString(evidence.preflightCheckedAt);
  const connectivityCheckedAt = canonicalISOString(evidence.connectivityCheckedAt);
  const startedAtMs = Date.parse(startedAt);
  const completedAtMs = Date.parse(completedAt);
  const preflightAtMs = Date.parse(preflightCheckedAt);
  const connectivityAtMs = Date.parse(connectivityCheckedAt);
  if (completedAtMs < startedAtMs || completedAtMs - startedAtMs > 10 * 60 * 1_000 ||
      preflightAtMs < startedAtMs || preflightAtMs > completedAtMs ||
      connectivityAtMs < preflightAtMs || connectivityAtMs > completedAtMs ||
      !validReadinessCount(evidence.profileCount, 64) || !validReadinessCount(evidence.credentialCount, 64) ||
      evidence.credentialCount > evidence.profileCount || !validReadinessCount(evidence.caBundleCount, 64) ||
      evidence.caBundleCount > evidence.profileCount || !validReadinessCount(evidence.toolCount, 256)) {
    throw new Error("Agent MCP readiness evidence is invalid");
  }
  return JSON.stringify({
    schemaVersion: evidence.schemaVersion, bindingSha256: evidence.bindingSha256,
    profileBindingSha256: evidence.profileBindingSha256, startedAt, completedAt,
    preflightCheckedAt, connectivityCheckedAt, profileCount: evidence.profileCount,
    credentialCount: evidence.credentialCount, caBundleCount: evidence.caBundleCount, toolCount: evidence.toolCount
  });
}

function canonicalISOString(value: string): string {
  const parsed = new Date(value);
  if (!Number.isFinite(parsed.getTime()) || parsed.toISOString() !== value) {
    throw new Error("Agent MCP readiness evidence time is invalid");
  }
  return value;
}

function validReadinessCount(value: number, maximum: number): boolean {
  return Number.isSafeInteger(value) && value >= 1 && value <= maximum;
}

function safeRevision(value: bigint): number {
  const revision = Number(value);
  if (!Number.isSafeInteger(revision) || revision < 0) {
    throw new Error("Agent Task Workflow revision exceeds the safe integer range");
  }
  return revision;
}

function validBoundedIdentifier(value: string, maximum: number): boolean {
  return value === value.trim() && value.length > 0 && value.length <= maximum;
}

function validSHA256(value: string): boolean {
  return /^[a-f0-9]{64}$/.test(value);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function assertSha256(value: string, label: string): void {
  if (!validSHA256(value)) throw new Error(`Agent MCP Tool ${label} is invalid`);
}

function safeUnixMilliseconds(value: bigint): number {
  const milliseconds = Number(value);
  if (!Number.isSafeInteger(milliseconds) || milliseconds <= 0) throw new Error("Agent timestamp is outside the safe JavaScript range");
  return milliseconds;
}

function agentResourceScopeSHA256(scope: { readonly resourceType: string; readonly resourceId: string; readonly actions: readonly string[] }): string {
  const resourceType = scope.resourceType.trim();
  const resourceId = scope.resourceId.trim();
  const actions = scope.actions.map(action => action.trim()).sort();
  if (!resourceType || !resourceId || actions.length === 0 || actions.some(action => !action) || new Set(actions).size !== actions.length) {
    throw new Error("Agent Approval resource scope is invalid");
  }
  return createHash("sha256").update(["dipole.agent.scope.v1", resourceType, resourceId, ...actions].join("\n")).digest("hex");
}

function sameAgentResourceScope(
  left: { readonly resourceType: string; readonly resourceId: string; readonly actions: readonly string[] },
  right: { readonly resourceType: string; readonly resourceId: string; readonly actions: readonly string[] }
): boolean {
  if (left.resourceType !== right.resourceType || left.resourceId !== right.resourceId) return false;
  const leftActions = [...left.actions].sort();
  const rightActions = [...right.actions].sort();
  return leftActions.length === rightActions.length && leftActions.every((action, index) => action === rightActions[index]);
}
