import { createHash } from "node:crypto";
import { isInputRequiredResult, type Tool, type Transport } from "@modelcontextprotocol/client";

import { canonicalMcpJSON } from "./canonical-json.js";
import type { McpElicitationResultInput } from "./mcp-durable-elicitation.js";
import {
  McpInputRequiredContinuation,
  type McpInputRequiredCheckpointV1
} from "./mcp-input-required-continuation.js";
import {
  AllowlistedMcpToolClient,
  type McpToolEgressPolicy,
  type McpToolRoundParams,
  type McpToolRoundResult
} from "./mcp-tool-client.js";

const checkpointSchemaVersion = "dipole.mcp.input-required-activity-checkpoint.v1" as const;
const identityPattern = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/;

export interface McpInputRequiredActivityCommand {
  readonly requestId: string;
  readonly tenantId: string;
  readonly profileId: string;
  readonly serverId: string;
  readonly toolName: string;
  readonly invocationId: string;
  readonly arguments: Readonly<Record<string, unknown>>;
  readonly expiresAtUnixMs: number;
}

export interface McpInputRequiredActivityCheckpointV1 {
  readonly schemaVersion: typeof checkpointSchemaVersion;
  readonly tenantId: string;
  readonly profileId: string;
  readonly serverId: string;
  readonly continuation: McpInputRequiredCheckpointV1;
  readonly bindingSha256: string;
}

export interface McpActivityRoundSession {
  callToolRound(params: McpToolRoundParams, signal?: AbortSignal): Promise<McpToolRoundResult>;
  close(): Promise<void>;
}

export interface McpActivityRoundSessionFactory {
  open(binding: {
    readonly tenantId: string;
    readonly profileId: string;
    readonly serverId: string;
    readonly toolName: string;
  }, signal?: AbortSignal): Promise<McpActivityRoundSession>;
}

interface McpActivityExternalProfile {
  readonly profileId: string;
  readonly tenantId: string;
  readonly serverId: string;
  readonly allowedTools: readonly string[];
}

export interface McpActivityExternalTransportRegistry {
  describe(profileId: string, tenantId: string): McpActivityExternalProfile;
  connect(profileId: string, tenantId: string, signal?: AbortSignal): Promise<Transport>;
}

export interface McpActivityModernClient extends McpActivityRoundSession {
  connect(transport: Transport): Promise<readonly Tool[]>;
}

interface McpActivityModernClientConfig {
  readonly protocolMode: "modern";
  readonly serverId: string;
  readonly allowedTools: readonly string[];
  readonly egressPolicies: Readonly<Record<string, McpToolEgressPolicy>>;
  readonly requestTimeoutMs: number;
}

type McpActivityModernClientFactory = (config: McpActivityModernClientConfig) => McpActivityModernClient;

export class ExternalMcpActivityRoundSessionFactory implements McpActivityRoundSessionFactory {
  readonly #registry: McpActivityExternalTransportRegistry;
  readonly #egressPolicies: Readonly<Record<string, Readonly<Record<string, McpToolEgressPolicy>>>>;
  readonly #requestTimeoutMs: number;
  readonly #createClient: McpActivityModernClientFactory;

  constructor(
    registry: McpActivityExternalTransportRegistry,
    egressPolicies: Readonly<Record<string, Readonly<Record<string, McpToolEgressPolicy>>>>,
    requestTimeoutMs = 10_000,
    createClient: McpActivityModernClientFactory = createModernClient
  ) {
    if (!Number.isSafeInteger(requestTimeoutMs) || requestTimeoutMs < 100 || requestTimeoutMs > 60_000) {
      throw new Error("MCP Activity request timeout must be between 100 and 60000 milliseconds");
    }
    this.#registry = registry;
    this.#egressPolicies = egressPolicies;
    this.#requestTimeoutMs = requestTimeoutMs;
    this.#createClient = createClient;
  }

  async open(
    binding: { readonly tenantId: string; readonly profileId: string; readonly serverId: string; readonly toolName: string },
    signal?: AbortSignal
  ): Promise<McpActivityRoundSession> {
    signal?.throwIfAborted();
    const profile = this.#registry.describe(binding.profileId, binding.tenantId);
    if (profile.serverId !== binding.serverId) throw new Error("MCP Activity Server binding does not match the external profile");
    if (!profile.allowedTools.includes(binding.toolName)) throw new Error("MCP Activity Tool is not allowed by the external profile");
    const policies = this.#egressPolicies[profile.profileId];
    if (policies === undefined) throw new Error("MCP Activity profile has no egress policies");
    const client = this.#createClient({
      protocolMode: "modern",
      serverId: profile.serverId,
      allowedTools: profile.allowedTools,
      egressPolicies: policies,
      requestTimeoutMs: this.#requestTimeoutMs
    });
    let transport: Transport | undefined;
    try {
      transport = await this.#registry.connect(profile.profileId, profile.tenantId, signal);
      signal?.throwIfAborted();
      await client.connect(transport);
      return client;
    } catch (error) {
      await Promise.allSettled([
        client.close(),
        ...(transport === undefined ? [] : [transport.close()])
      ]);
      throw error;
    }
  }
}

export type McpInputRequiredActivityResult =
  | { readonly kind: "complete"; readonly result: Exclude<McpToolRoundResult, { resultType: "input_required" }> }
  | {
    readonly kind: "wait_input";
    readonly directive: ReturnType<McpInputRequiredContinuation["begin"]>["directive"];
    readonly checkpoint: McpInputRequiredActivityCheckpointV1;
  };

export class McpInputRequiredActivity {
  readonly #sessions: McpActivityRoundSessionFactory;
  readonly #continuation: McpInputRequiredContinuation;

  constructor(sessions: McpActivityRoundSessionFactory, now: () => number = Date.now) {
    this.#sessions = sessions;
    this.#continuation = new McpInputRequiredContinuation(now);
  }

  async begin(command: McpInputRequiredActivityCommand, signal?: AbortSignal): Promise<McpInputRequiredActivityResult> {
    validateCommand(command);
    const result = await this.#withSession(command, {
      name: command.toolName,
      arguments: command.arguments
    }, signal);
    if (!isInputRequiredResult(result)) return { kind: "complete", result };

    const wait = this.#continuation.begin({
      result,
      requestId: command.requestId,
      serverId: command.serverId,
      toolName: command.toolName,
      invocationId: command.invocationId,
      arguments: command.arguments,
      expiresAtUnixMs: command.expiresAtUnixMs
    });
    const binding = {
      schemaVersion: checkpointSchemaVersion,
      tenantId: command.tenantId,
      profileId: command.profileId,
      serverId: command.serverId,
      continuation: wait.checkpoint
    };
    const checkpoint: McpInputRequiredActivityCheckpointV1 = {
      ...binding,
      bindingSha256: sha256(canonicalMcpJSON(binding))
    };
    return {
      kind: "wait_input",
      checkpoint,
      directive: { ...wait.directive, checkpoint }
    };
  }

  async resume(
    checkpoint: McpInputRequiredActivityCheckpointV1,
    input: McpElicitationResultInput,
    signal?: AbortSignal
  ): Promise<Extract<McpInputRequiredActivityResult, { kind: "complete" }>> {
    validateCheckpoint(checkpoint);
    const retry = this.#continuation.retry(checkpoint.continuation, input);
    const result = await this.#withSession({
      tenantId: checkpoint.tenantId,
      profileId: checkpoint.profileId,
      serverId: checkpoint.serverId,
      toolName: checkpoint.continuation.toolName
    }, retry, signal);
    if (isInputRequiredResult(result)) {
      throw new Error("MCP Activity does not support an additional input_required round");
    }
    return { kind: "complete", result };
  }

  async #withSession(
    binding: { readonly tenantId: string; readonly profileId: string; readonly serverId: string; readonly toolName: string },
    params: McpToolRoundParams,
    signal?: AbortSignal
  ): Promise<McpToolRoundResult> {
    signal?.throwIfAborted();
    const session = await this.#sessions.open(binding, signal);
    try {
      signal?.throwIfAborted();
      return await session.callToolRound(params, signal);
    } finally {
      await session.close();
    }
  }
}

function validateCommand(command: McpInputRequiredActivityCommand): void {
  for (const [label, value] of [
    ["tenant", command.tenantId], ["profile", command.profileId], ["server", command.serverId]
  ] as const) {
    if (!identityPattern.test(value)) throw new Error(`MCP Activity ${label} identity is invalid`);
  }
}

function validateCheckpoint(checkpoint: McpInputRequiredActivityCheckpointV1): void {
  if (!isRecord(checkpoint) || checkpoint.schemaVersion !== checkpointSchemaVersion ||
      !identityPattern.test(checkpoint.tenantId) || !identityPattern.test(checkpoint.profileId) ||
      !identityPattern.test(checkpoint.serverId) || !isRecord(checkpoint.continuation)) {
    throw new Error("MCP Activity checkpoint is invalid");
  }
  const { bindingSha256, ...binding } = checkpoint;
  if (typeof bindingSha256 !== "string" || bindingSha256 !== sha256(canonicalMcpJSON(binding))) {
    throw new Error("MCP Activity checkpoint integrity validation failed");
  }
  if (checkpoint.serverId !== checkpoint.continuation.serverId) {
    throw new Error("MCP Activity checkpoint Server lineage is invalid");
  }
}

function sha256(value: string): string {
  return createHash("sha256").update(value, "utf8").digest("hex");
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function createModernClient(config: McpActivityModernClientConfig): McpActivityModernClient {
  return new AllowlistedMcpToolClient(
    config.serverId,
    [config.serverId],
    config.allowedTools,
    config.egressPolicies,
    config.requestTimeoutMs,
    config.protocolMode
  );
}
