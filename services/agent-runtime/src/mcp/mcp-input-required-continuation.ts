import { createHash } from "node:crypto";

import type { AgentTaskDirective } from "../temporal/agent-task-activities.js";
import { canonicalMcpJSON } from "./canonical-json.js";
import {
  McpDurableElicitationAdapter,
  type McpElicitationCheckpointV1,
  type McpElicitationResultInput
} from "./mcp-durable-elicitation.js";

const checkpointSchemaVersion = "dipole.mcp.input-required-checkpoint.v1" as const;
const inputKeyPattern = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/;
const maximumArgumentsBytes = 16 * 1024;
const maximumRequestStateBytes = 8 * 1024;
const sensitiveArgumentNames = new Set([
  "password", "passwd", "secret", "secretkey", "token", "apikey", "apitoken", "authkey", "authtoken",
  "accesskey", "accesstoken", "refreshtoken", "sessionid", "sessiontoken", "bearertoken", "authorization",
  "cookie", "credential", "credentials", "privatekey", "clientsecret", "payment", "creditcard"
]);

export interface McpInputRequiredCheckpointV1 {
  readonly schemaVersion: typeof checkpointSchemaVersion;
  readonly requestId: string;
  readonly serverId: string;
  readonly toolName: string;
  readonly invocationId: string;
  readonly inputRequestKey: string;
  readonly arguments: Readonly<Record<string, unknown>>;
  readonly requestState?: string;
  readonly elicitation: McpElicitationCheckpointV1;
  readonly bindingSha256: string;
}

export interface McpInputRequiredRetryParams {
  readonly name: string;
  readonly arguments: Readonly<Record<string, unknown>>;
  readonly inputResponses: Readonly<Record<string, unknown>>;
  readonly requestState?: string;
}

export class McpInputRequiredContinuation {
  readonly #elicitation: McpDurableElicitationAdapter;

  constructor(now: () => number = Date.now) {
    this.#elicitation = new McpDurableElicitationAdapter(now);
  }

  begin(input: {
    readonly result: unknown;
    readonly requestId: string;
    readonly serverId: string;
    readonly toolName: string;
    readonly invocationId: string;
    readonly arguments: Readonly<Record<string, unknown>>;
    readonly expiresAtUnixMs: number;
  }): { directive: Extract<AgentTaskDirective, { kind: "wait_input" }>; checkpoint: McpInputRequiredCheckpointV1 } {
    const round = parseInputRequired(input.result);
    const argumentsSnapshot = snapshotArguments(input.arguments);
    const elicitation = this.#elicitation.request({
      request: round.request,
      requestId: input.requestId,
      serverId: input.serverId,
      toolName: input.toolName,
      invocationId: input.invocationId,
      expiresAtUnixMs: input.expiresAtUnixMs
    });
    const binding = {
      schemaVersion: checkpointSchemaVersion,
      requestId: input.requestId,
      serverId: input.serverId,
      toolName: input.toolName,
      invocationId: input.invocationId,
      inputRequestKey: round.inputRequestKey,
      arguments: argumentsSnapshot,
      ...(round.requestState === undefined ? {} : { requestState: round.requestState }),
      elicitation: elicitation.checkpoint
    };
    const checkpoint: McpInputRequiredCheckpointV1 = {
      ...binding,
      bindingSha256: sha256(canonicalMcpJSON(binding))
    };
    return {
      checkpoint,
      directive: { ...elicitation.directive, checkpoint }
    };
  }

  retry(checkpoint: McpInputRequiredCheckpointV1, input: McpElicitationResultInput): McpInputRequiredRetryParams {
    validateCheckpoint(checkpoint);
    const response = this.#elicitation.result(checkpoint.elicitation, input);
    return {
      name: checkpoint.toolName,
      arguments: snapshotArguments(checkpoint.arguments),
      inputResponses: { [checkpoint.inputRequestKey]: response },
      ...(checkpoint.requestState === undefined ? {} : { requestState: checkpoint.requestState })
    };
  }
}

function parseInputRequired(raw: unknown): { inputRequestKey: string; request: unknown; requestState?: string } {
  if (!isRecord(raw) || raw.resultType !== "input_required") throw new Error("MCP input_required result is invalid");
  rejectUnknownKeys(raw, ["resultType", "inputRequests", "requestState"], "MCP input_required result");
  if (!isRecord(raw.inputRequests) || Object.keys(raw.inputRequests).length !== 1) {
    throw new Error("MCP continuation requires exactly one input request");
  }
  const entry = Object.entries(raw.inputRequests)[0]!;
  if (!inputKeyPattern.test(entry[0])) throw new Error("MCP input request key is invalid");
  if (raw.requestState !== undefined && (typeof raw.requestState !== "string" || Buffer.byteLength(raw.requestState, "utf8") > maximumRequestStateBytes)) {
    throw new Error("MCP requestState is invalid or exceeds 8 KiB");
  }
  return {
    inputRequestKey: entry[0],
    request: entry[1],
    ...(raw.requestState === undefined ? {} : { requestState: raw.requestState })
  };
}

function validateCheckpoint(checkpoint: McpInputRequiredCheckpointV1): void {
  if (!isRecord(checkpoint) || checkpoint.schemaVersion !== checkpointSchemaVersion ||
      !inputKeyPattern.test(checkpoint.inputRequestKey) || !isRecord(checkpoint.elicitation)) {
    throw new Error("MCP continuation checkpoint is invalid");
  }
  const { bindingSha256, ...binding } = checkpoint;
  if (typeof bindingSha256 !== "string" || bindingSha256 !== sha256(canonicalMcpJSON(binding))) {
    throw new Error("MCP continuation checkpoint integrity validation failed");
  }
  if (checkpoint.requestId !== checkpoint.elicitation.requestId || checkpoint.serverId !== checkpoint.elicitation.serverId ||
      checkpoint.toolName !== checkpoint.elicitation.toolName || checkpoint.invocationId !== checkpoint.elicitation.invocationId) {
    throw new Error("MCP continuation checkpoint lineage is invalid");
  }
  if (checkpoint.requestState !== undefined && Buffer.byteLength(checkpoint.requestState, "utf8") > maximumRequestStateBytes) {
    throw new Error("MCP continuation checkpoint requestState is invalid");
  }
  snapshotArguments(checkpoint.arguments);
}

function snapshotArguments(raw: unknown): Readonly<Record<string, unknown>> {
  if (!isRecord(raw)) throw new Error("MCP continuation arguments must be an object");
  rejectCredentialFields(raw, 0);
  const encoded = canonicalMcpJSON(raw);
  if (Buffer.byteLength(encoded, "utf8") > maximumArgumentsBytes) throw new Error("MCP continuation arguments exceed 16 KiB");
  return JSON.parse(encoded) as Record<string, unknown>;
}

function rejectCredentialFields(value: unknown, depth: number): void {
  if (depth > 16) throw new Error("MCP continuation arguments are too deeply nested");
  if (Array.isArray(value)) {
    for (const item of value) rejectCredentialFields(item, depth + 1);
    return;
  }
  if (!isRecord(value)) return;
  for (const [key, item] of Object.entries(value)) {
    if (sensitiveArgumentNames.has(key.toLowerCase().replace(/[^a-z0-9]/g, ""))) {
      throw new Error("MCP continuation arguments contain a credential field");
    }
    rejectCredentialFields(item, depth + 1);
  }
}

function rejectUnknownKeys(value: Record<string, unknown>, allowed: readonly string[], label: string): void {
  const unknown = Object.keys(value).find(key => !allowed.includes(key));
  if (unknown !== undefined) throw new Error(`${label} contains unsupported field ${unknown}`);
}

function sha256(value: string): string {
  return createHash("sha256").update(value, "utf8").digest("hex");
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
