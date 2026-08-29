import { createHash } from "node:crypto";
import { specTypeSchemas } from "@modelcontextprotocol/client";

import type {
  AgentArtifactCreateInput,
  AgentArtifactRecord,
  AgentMcpToolCommand
} from "../capabilities/agent-capability-rpc.js";
import { canonicalMcpJSON } from "./canonical-json.js";
import { executionContextSchema, type ExecutionContext } from "../runtime/execution-context.js";

const schemaVersion = "dipole.agent.external-mcp-result.v1" as const;
const artifactVersion = 1 as const;
const maximumResultBytes = 128 * 1024;
const sha256Pattern = /^[a-f0-9]{64}$/;
const identityPattern = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/;
const shortIdentityPattern = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$/;
const toolNamePattern = /^[A-Za-z][A-Za-z0-9_.-]{0,63}$/;

export interface ExternalMcpArtifactCommandResolver {
  resolveMcpToolCommand(taskId: string, runId: string, invocationId: string): Promise<AgentMcpToolCommand>;
}

export interface ExternalMcpArtifactWriter {
  createArtifact(input: AgentArtifactCreateInput): Promise<AgentArtifactRecord>;
}

export interface ExternalMcpArtifactProjectionInput {
  readonly context: ExecutionContext;
  readonly invocationId: string;
  readonly roundId: string;
  readonly result: unknown;
}

export class ExternalMcpArtifactProjector {
  constructor(
    private readonly commands: ExternalMcpArtifactCommandResolver,
    private readonly artifacts: ExternalMcpArtifactWriter
  ) {}

  async project(
    rawInput: ExternalMcpArtifactProjectionInput,
    signal?: AbortSignal
  ): Promise<{ readonly artifactId: string; readonly artifactVersion: number }> {
    signal?.throwIfAborted();
    const input = validateInput(rawInput);
    const command = await this.commands.resolveMcpToolCommand(
      input.context.taskId,
      input.context.runId,
      input.invocationId
    );
    signal?.throwIfAborted();
    validateCommand(command, input);

    const contentJSON = validateResult(input.result);
    const content = Buffer.from(contentJSON, "utf8");
    const resultSha256 = sha256(content);
    const artifactType = artifactTypeFor(input.invocationId);
    const metadata = {
      schemaVersion,
      trust: "untrusted",
      sourceType: "external_mcp_tool",
      invocationId: input.invocationId,
      roundId: input.roundId,
      profileId: command.profileId,
      serverId: command.serverId,
      toolName: command.toolName,
      capabilityId: command.capabilityId,
      argumentsSha256: command.argumentsSha256,
      resultSha256
    } as const;
    const title = `External MCP result: ${command.serverId}/${command.toolName}`;
    signal?.throwIfAborted();
    const artifact = await this.artifacts.createArtifact({
      tenantId: command.tenantId,
      taskId: command.taskId,
      runId: command.runId,
      artifactType,
      version: artifactVersion,
      title,
      mediaType: "application/json",
      content,
      metadata,
      ...(input.context.requestId === undefined ? {} : { requestId: input.context.requestId }),
      ...(input.context.traceId === undefined ? {} : { traceId: input.context.traceId })
    });
    signal?.throwIfAborted();
    validateArtifact(artifact, {
      taskId: command.taskId,
      runId: command.runId,
      artifactType,
      title,
      content,
      resultSha256,
      metadata
    });
    return { artifactId: artifact.artifactId, artifactVersion: artifact.version };
  }
}

function validateInput(input: ExternalMcpArtifactProjectionInput): ExternalMcpArtifactProjectionInput {
  const parsedContext = executionContextSchema.safeParse(input.context);
  if (!parsedContext.success || parsedContext.data.mode !== "shadow" ||
      !sha256Pattern.test(input.invocationId) || !sha256Pattern.test(input.roundId)) {
    throw new Error("External MCP Artifact projection input is invalid");
  }
  return { ...input, context: parsedContext.data };
}

function validateCommand(command: AgentMcpToolCommand, input: ExternalMcpArtifactProjectionInput): void {
  if (command.status !== "completed" || command.invocationId !== input.invocationId ||
      command.taskId !== input.context.taskId || command.runId !== input.context.runId ||
      command.tenantId !== input.context.tenantId || command.principalUserId !== input.context.principalUuid ||
      command.agentId !== input.context.agentUuid || !identityPattern.test(command.tenantId) ||
      !identityPattern.test(command.principalUserId) || !identityPattern.test(command.agentId) ||
      !shortIdentityPattern.test(command.profileId) || !shortIdentityPattern.test(command.serverId) ||
      !toolNamePattern.test(command.toolName) || !identityPattern.test(command.capabilityId) ||
      !sha256Pattern.test(command.argumentsSha256)) {
    throw new Error("External MCP Artifact command binding is invalid");
  }
}

function validateResult(raw: unknown): string {
  const result = specTypeSchemas.CallToolResult["~standard"].validate(raw);
  if (result.issues !== undefined || result.value.isError === true) {
    throw new Error("External MCP Artifact requires a successful Tool result");
  }
  let rawJSON: string;
  let parsedJSON: string;
  try {
    rawJSON = canonicalMcpJSON(raw);
    parsedJSON = canonicalMcpJSON(result.value);
  } catch {
    throw new Error("External MCP Artifact result must be canonical JSON");
  }
  if (rawJSON !== parsedJSON || Buffer.byteLength(rawJSON, "utf8") > maximumResultBytes) {
    throw new Error("External MCP Artifact result is transformed or exceeds 128 KiB");
  }
  return rawJSON;
}

function artifactTypeFor(invocationId: string): string {
  return `mcp_result.${invocationId.slice(0, 53)}`;
}

function validateArtifact(
  artifact: AgentArtifactRecord,
  expected: {
    readonly taskId: string;
    readonly runId: string;
    readonly artifactType: string;
    readonly title: string;
    readonly content: Uint8Array;
    readonly resultSha256: string;
    readonly metadata: Readonly<Record<string, unknown>>;
  }
): void {
  const expectedArtifactId = sha256(Buffer.from([
    "dipole.agent.artifact.v1",
    expected.taskId,
    expected.runId,
    expected.artifactType,
    artifactVersion.toString(),
    expected.resultSha256
  ].join("\n"), "utf8"));
  if (artifact.schemaVersion !== "dipole.agent.artifact.v1" || artifact.artifactId !== expectedArtifactId ||
      artifact.taskId !== expected.taskId || artifact.runId !== expected.runId ||
      artifact.artifactType !== expected.artifactType || artifact.version !== artifactVersion ||
      artifact.title !== expected.title || artifact.mediaType !== "application/json" ||
      artifact.contentSha256 !== expected.resultSha256 || artifact.sizeBytes !== expected.content.byteLength ||
      canonicalMcpJSON(artifact.metadata) !== canonicalMcpJSON(expected.metadata)) {
    throw new Error("External MCP Artifact writer returned conflicting evidence");
  }
}

function sha256(value: Uint8Array): string {
  return createHash("sha256").update(value).digest("hex");
}
