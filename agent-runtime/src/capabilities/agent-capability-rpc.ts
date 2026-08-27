import * as grpc from "@grpc/grpc-js";

import type { AgentEvent, AgentIdentity } from "../events/shadow-processor.js";
import type { IAgentCapabilityServiceClient } from "../generated/dipole/agent/v1/agent.grpc-client.js";
import type { ConversationSnapshot } from "../generated/dipole/agent/v1/agent.js";
import { executionContextSchema, type ExecutionContext } from "../runtime/execution-context.js";
import type { AgentEventSubscription } from "../events/event-subscription.js";
import { createHash } from "node:crypto";

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
          resolve(executionContextSchema.parse({
            tenantId: response.tenantId, principalUuid: response.principalUserId, agentUuid: response.agentId,
            ...(response.delegatedByUserId.trim() === "" ? {} : { delegatedByUuid: response.delegatedByUserId }),
            taskId, runId, mode: "shadow", permissions: response.permissions,
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
    const metadata = this.metadata(input.requestId, input.traceId);
    return new Promise((resolve, reject) => {
      this.rpc.beginMcpToolInvocation({
        context: this.requestContext(input.requestId, input.traceId), taskId: input.taskId, runId: input.runId,
        invocationId: input.invocationId, toolName: input.toolName, capabilityId: input.capabilityId,
        argumentsSha256: input.argumentsSha256, approvalId: input.approvalId ?? ""
      }, metadata, { deadline: Date.now() + this.timeoutMs }, (error, response) => {
        if (error !== null || response === undefined) return reject(error ?? new Error("Agent Tool invocation begin returned no response"));
        if (response.invocationId !== input.invocationId || response.status !== "running") return reject(new Error("Agent Tool invocation begin returned conflicting evidence"));
        resolve();
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
