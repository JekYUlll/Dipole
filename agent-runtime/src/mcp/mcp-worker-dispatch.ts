import { createHash } from "node:crypto";
import { z } from "zod";

import type { AgentMcpToolCommand } from "../capabilities/agent-capability-rpc.js";
import { canonicalMcpJSON } from "./canonical-json.js";
import type { McpElicitationResultInput } from "./mcp-durable-elicitation.js";
import {
  type McpInputRequiredActivityCheckpointV1,
  type McpInputRequiredActivityResult
} from "./mcp-input-required-activity.js";

const checkpointSchemaVersion = "dipole.mcp.worker-dispatch-checkpoint.v1" as const;
const identitySchema = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/);
const sha256Schema = z.string().regex(/^[a-f0-9]{64}$/);
const dispatchInputSchema = z.object({
  taskId: identitySchema,
  runId: identitySchema,
  invocationId: identitySchema
}).strict();
const dispatchCheckpointSchema = z.object({
  schemaVersion: z.literal(checkpointSchemaVersion),
  taskId: identitySchema,
  runId: identitySchema,
  invocationId: identitySchema,
  commandBindingSha256: sha256Schema,
  activity: z.unknown(),
  bindingSha256: sha256Schema
}).strict();

type ActivityWait = Extract<McpInputRequiredActivityResult, { kind: "wait_input" }>;

export interface McpWorkerDispatchCheckpointV1 {
  readonly schemaVersion: typeof checkpointSchemaVersion;
  readonly taskId: string;
  readonly runId: string;
  readonly invocationId: string;
  readonly commandBindingSha256: string;
  readonly activity: McpInputRequiredActivityCheckpointV1;
  readonly bindingSha256: string;
}

export type McpWorkerDispatchResult =
  | Extract<McpInputRequiredActivityResult, { kind: "complete" }>
  | {
    readonly kind: "wait_input";
    readonly checkpoint: McpWorkerDispatchCheckpointV1;
    readonly directive: Omit<ActivityWait["directive"], "checkpoint"> & { readonly checkpoint: McpWorkerDispatchCheckpointV1 };
  };

interface McpWorkerCommandResolver {
  resolveMcpToolCommand(taskId: string, runId: string, invocationId: string): Promise<AgentMcpToolCommand>;
}

interface McpWorkerActivity {
  begin(command: {
    readonly requestId: string; readonly taskId: string; readonly runId: string; readonly tenantId: string;
    readonly profileId: string; readonly serverId: string; readonly toolName: string; readonly invocationId: string;
    readonly arguments: Readonly<Record<string, unknown>>; readonly expiresAtUnixMs: number;
  }, signal?: AbortSignal): Promise<McpInputRequiredActivityResult>;
  resume(checkpoint: McpInputRequiredActivityCheckpointV1, input: McpElicitationResultInput, signal?: AbortSignal): Promise<Extract<McpInputRequiredActivityResult, { kind: "complete" }>>;
}

export class McpWorkerCommandDispatcher {
  constructor(
    private readonly resolver: McpWorkerCommandResolver,
    private readonly activity: McpWorkerActivity,
    private readonly now: () => number = Date.now,
    private readonly inputWindowMs = 15 * 60 * 1_000
  ) {
    if (!Number.isSafeInteger(inputWindowMs) || inputWindowMs < 60_000 || inputWindowMs > 60 * 60 * 1_000) {
      throw new Error("MCP Worker input window must be between 1 minute and 1 hour");
    }
  }

  async begin(rawInput: unknown, signal?: AbortSignal): Promise<McpWorkerDispatchResult> {
    const parsed = dispatchInputSchema.safeParse(rawInput);
    if (!parsed.success) throw new Error("MCP Worker dispatch input is invalid");
    const input = parsed.data;
    const command = await this.resolve(input.taskId, input.runId, input.invocationId);
    const expiresAtUnixMs = command.startedAtUnixMs + this.inputWindowMs;
    const now = this.now();
    if (command.startedAtUnixMs > now + 60_000 || expiresAtUnixMs <= now) {
      throw new Error("MCP Worker command execution window is invalid");
    }
    const result = await this.activity.begin({
      requestId: sha256(`dipole.mcp.worker-input.v1\n${command.invocationId}`),
      taskId: command.taskId, runId: command.runId, tenantId: command.tenantId,
      profileId: command.profileId, serverId: command.serverId, toolName: command.toolName,
      invocationId: command.invocationId, arguments: command.arguments, expiresAtUnixMs
    }, signal);
    if (result.kind === "complete") return result;
    const checkpoint = createDispatchCheckpoint(command, result.checkpoint);
    return {
      kind: "wait_input",
      checkpoint,
      directive: { ...result.directive, checkpoint }
    };
  }

  async resume(rawCheckpoint: unknown, input: McpElicitationResultInput, signal?: AbortSignal): Promise<Extract<McpWorkerDispatchResult, { kind: "complete" }>> {
    const checkpoint = parseDispatchCheckpoint(rawCheckpoint);
    const command = await this.resolve(checkpoint.taskId, checkpoint.runId, checkpoint.invocationId);
    if (commandBindingSha256(command) !== checkpoint.commandBindingSha256 || !sameActivityBinding(command, checkpoint.activity)) {
      throw new Error("MCP Worker command binding changed before resume");
    }
    return this.activity.resume(checkpoint.activity, input, signal);
  }

  private async resolve(taskId: string, runId: string, invocationId: string): Promise<AgentMcpToolCommand> {
    const command = await this.resolver.resolveMcpToolCommand(taskId, runId, invocationId);
    if (command.taskId !== taskId || command.runId !== runId || command.invocationId !== invocationId) {
      throw new Error("MCP Worker command binding is invalid");
    }
    commandBindingSha256(command);
    return command;
  }
}

function createDispatchCheckpoint(command: AgentMcpToolCommand, activity: McpInputRequiredActivityCheckpointV1): McpWorkerDispatchCheckpointV1 {
  const binding = {
    schemaVersion: checkpointSchemaVersion,
    taskId: command.taskId,
    runId: command.runId,
    invocationId: command.invocationId,
    commandBindingSha256: commandBindingSha256(command),
    activity
  };
  return { ...binding, bindingSha256: sha256(canonicalMcpJSON(binding)) };
}

function parseDispatchCheckpoint(raw: unknown): McpWorkerDispatchCheckpointV1 {
  const parsed = dispatchCheckpointSchema.safeParse(raw);
  if (!parsed.success) throw new Error("MCP Worker dispatch checkpoint is invalid");
  const { bindingSha256, ...binding } = parsed.data;
  if (sha256(canonicalMcpJSON(binding)) !== bindingSha256) {
    throw new Error("MCP Worker dispatch checkpoint integrity validation failed");
  }
  return parsed.data as McpWorkerDispatchCheckpointV1;
}

function commandBindingSha256(command: AgentMcpToolCommand): string {
  if (!isRecord(command.arguments)) throw new Error("MCP Worker command binding is invalid");
  const argumentsJSON = canonicalMcpJSON(command.arguments);
  if (sha256(argumentsJSON) !== command.argumentsSha256 || !Number.isSafeInteger(command.startedAtUnixMs) || command.startedAtUnixMs <= 0) {
    throw new Error("MCP Worker command binding is invalid");
  }
  const binding = {
    invocationId: command.invocationId, tenantId: command.tenantId, principalUserId: command.principalUserId,
    agentId: command.agentId, taskId: command.taskId, runId: command.runId, profileId: command.profileId,
    serverId: command.serverId, toolName: command.toolName, capabilityId: command.capabilityId,
    argumentsSha256: command.argumentsSha256, startedAtUnixMs: command.startedAtUnixMs
  };
  for (const value of Object.values(binding)) {
    if (typeof value === "string" && !identitySchema.safeParse(value).success) {
      throw new Error("MCP Worker command binding is invalid");
    }
  }
  return sha256(canonicalMcpJSON(binding));
}

function sameActivityBinding(command: AgentMcpToolCommand, activity: McpInputRequiredActivityCheckpointV1): boolean {
  return activity.taskId === command.taskId && activity.runId === command.runId &&
    activity.tenantId === command.tenantId && activity.profileId === command.profileId && activity.serverId === command.serverId &&
    activity.continuation.invocationId === command.invocationId && activity.continuation.toolName === command.toolName &&
    canonicalMcpJSON(activity.continuation.arguments) === canonicalMcpJSON(command.arguments);
}

function sha256(value: string): string {
  return createHash("sha256").update(value, "utf8").digest("hex");
}

function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
