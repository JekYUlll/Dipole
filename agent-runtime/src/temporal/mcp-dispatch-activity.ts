import { createHash } from "node:crypto";
import { Context as TemporalActivityContext } from "@temporalio/activity";
import { z } from "zod";

import type { AgentTaskDirective } from "./agent-task-activities.js";
import type { AgentTaskResume } from "../task/agent-task-state.js";
import type { ExecutionContext } from "../runtime/execution-context.js";
import type { McpInvocationProducerResult } from "../mcp/mcp-invocation-producer.js";
import type { McpWorkerDispatchResult } from "../mcp/mcp-worker-dispatch.js";
import type { McpElicitationResultInput } from "../mcp/mcp-durable-elicitation.js";
import { canonicalMcpJSON } from "../mcp/canonical-json.js";

const checkpointSchemaVersion = "dipole.mcp.temporal-dispatch-checkpoint.v1" as const;
const identitySchema = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/);
const coreIdentitySchema = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$/);
const sha256Schema = z.string().regex(/^[a-f0-9]{64}$/);
const beginInputSchema = z.object({
  kind: z.literal("begin"),
  routeId: identitySchema,
  routeVersion: z.number().int().min(1).max(1_000_000),
  taskId: coreIdentitySchema,
  runId: coreIdentitySchema,
  principalUserId: identitySchema,
  arguments: z.unknown(),
  requestId: identitySchema.optional(),
  traceId: identitySchema.optional()
}).strict();
const resumeInputSchema = z.object({
  kind: z.literal("resume"),
  checkpoint: z.unknown(),
  resume: z.unknown()
}).strict();
const checkpointSchema = z.object({
  schemaVersion: z.literal(checkpointSchemaVersion),
  taskId: coreIdentitySchema,
  runId: coreIdentitySchema,
  principalUserId: identitySchema,
  routeId: identitySchema,
  routeVersion: z.number().int().min(1).max(1_000_000),
  capabilityId: identitySchema,
  workflowStep: z.number().int().min(0).max(255),
  ordinal: z.number().int().min(0).max(255),
  invocationId: sha256Schema,
  arguments: z.record(z.string(), z.unknown()),
  workerCheckpoint: z.unknown(),
  requestId: identitySchema.optional(),
  traceId: identitySchema.optional(),
  bindingSha256: sha256Schema
}).strict();
const artifactProjectionSchema = z.object({
  artifactId: sha256Schema,
  artifactVersion: z.number().int().min(1).max(1_000_000)
}).strict();

export interface TemporalMcpDispatchRoute {
  readonly routeId: string;
  readonly routeVersion: number;
  readonly capabilityId: string;
  readonly workflowStep: number;
  readonly ordinal: number;
}

export interface TemporalMcpDispatchCheckpointV1 {
  readonly schemaVersion: typeof checkpointSchemaVersion;
  readonly taskId: string;
  readonly runId: string;
  readonly principalUserId: string;
  readonly routeId: string;
  readonly routeVersion: number;
  readonly capabilityId: string;
  readonly workflowStep: number;
  readonly ordinal: number;
  readonly invocationId: string;
  readonly arguments: Readonly<Record<string, unknown>>;
  readonly workerCheckpoint: unknown;
  readonly requestId?: string;
  readonly traceId?: string;
  readonly bindingSha256: string;
}

export type TemporalMcpDispatchActivityInput =
  | {
    readonly kind: "begin";
    readonly routeId: string;
    readonly routeVersion: number;
    readonly taskId: string;
    readonly runId: string;
    readonly principalUserId: string;
    readonly arguments: unknown;
    readonly requestId?: string;
    readonly traceId?: string;
  }
  | {
    readonly kind: "resume";
    readonly checkpoint: TemporalMcpDispatchCheckpointV1;
    readonly resume: AgentTaskResume;
  };

export interface TemporalMcpContextResolver {
  resolveMcpContext(
    taskId: string,
    runId: string,
    principalUserId: string,
    correlation?: { readonly requestId?: string; readonly traceId?: string }
  ): Promise<ExecutionContext>;
}

export interface TemporalMcpInvocationProducer {
  produce(input: unknown, context: ExecutionContext): Promise<McpInvocationProducerResult>;
}

export interface TemporalMcpTerminalWorker {
  begin(input: unknown, signal?: AbortSignal): Promise<McpWorkerDispatchResult>;
  resume(checkpoint: unknown, input: McpElicitationResultInput, signal?: AbortSignal): Promise<Extract<McpWorkerDispatchResult, { kind: "complete" }>>;
}

export interface TemporalMcpResultProjector {
  project(input: {
    readonly context: ExecutionContext;
    readonly invocationId: string;
    readonly roundId: string;
    readonly result: unknown;
  }, signal?: AbortSignal): Promise<{ readonly artifactId: string; readonly artifactVersion: number }>;
}

export interface TemporalMcpDispatchDependencies {
  readonly contexts: TemporalMcpContextResolver;
  readonly producer: TemporalMcpInvocationProducer;
  readonly worker: TemporalMcpTerminalWorker;
  readonly projector: TemporalMcpResultProjector;
  readonly cancellationSignal?: () => AbortSignal;
}

export interface TemporalMcpDispatchActivities {
  executeMcpDispatch(input: TemporalMcpDispatchActivityInput): Promise<AgentTaskDirective>;
}

export function createTemporalMcpDispatchActivities(
  route: TemporalMcpDispatchRoute,
  dependencies: TemporalMcpDispatchDependencies
): TemporalMcpDispatchActivities {
  const activity = new TemporalMcpDispatchActivity(route, dependencies);
  return {
    executeMcpDispatch: input => activity.execute(input)
  };
}

export class TemporalMcpDispatchActivity {
  readonly #route: TemporalMcpDispatchRoute;
  readonly #dependencies: TemporalMcpDispatchDependencies;

  constructor(route: TemporalMcpDispatchRoute, dependencies: TemporalMcpDispatchDependencies) {
    const parsed = z.object({
      routeId: identitySchema,
      routeVersion: z.number().int().min(1).max(1_000_000),
      capabilityId: identitySchema,
      workflowStep: z.number().int().min(0).max(255),
      ordinal: z.number().int().min(0).max(255)
    }).strict().parse(route);
    this.#route = parsed;
    this.#dependencies = dependencies;
  }

  async execute(rawInput: unknown): Promise<AgentTaskDirective> {
    const signal = (this.#dependencies.cancellationSignal ?? temporalCancellationSignal)();
    signal.throwIfAborted();
    const input = parseInput(rawInput);
    if (input.kind === "begin") {
      if (input.routeId !== this.#route.routeId || input.routeVersion !== this.#route.routeVersion) {
        throw new Error("Temporal MCP dispatch route binding is invalid");
      }
      const argumentsValue = canonicalArguments(input.arguments);
      const context = await this.resolveContext(input.taskId, input.runId, input.principalUserId, correlation(input), signal);
      const produced = await this.produce(argumentsValue, context, signal);
      const result = await this.#dependencies.worker.begin({
        taskId: produced.taskId, runId: produced.runId, invocationId: produced.invocationId
      }, signal);
      signal.throwIfAborted();
      return this.projectResult(result, context, produced, argumentsValue, input, signal);
    }

    const checkpoint = parseCheckpoint(input.checkpoint, this.#route);
    const context = await this.resolveContext(
      checkpoint.taskId, checkpoint.runId, checkpoint.principalUserId, correlation(checkpoint), signal
    );
    const produced = await this.produce(checkpoint.arguments, context, signal);
    if (produced.invocationId !== checkpoint.invocationId) {
      throw new Error("Temporal MCP dispatch Invocation binding changed before resume");
    }
    const result = await this.#dependencies.worker.resume(
      checkpoint.workerCheckpoint,
      toElicitationResult(input.resume),
      signal
    );
    signal.throwIfAborted();
    return this.projectComplete(result, context, produced, signal);
  }

  private async resolveContext(
    taskId: string,
    runId: string,
    principalUserId: string,
    correlationValue: { readonly requestId?: string; readonly traceId?: string },
    signal: AbortSignal
  ): Promise<ExecutionContext> {
    const context = await this.#dependencies.contexts.resolveMcpContext(
      taskId, runId, principalUserId, correlationValue
    );
    signal.throwIfAborted();
    if (context.taskId !== taskId || context.runId !== runId || context.principalUuid !== principalUserId) {
      throw new Error("Temporal MCP dispatch Context binding is invalid");
    }
    return context;
  }

  private async produce(
    argumentsValue: Readonly<Record<string, unknown>>,
    context: ExecutionContext,
    signal: AbortSignal
  ): Promise<McpInvocationProducerResult> {
    const produced = await this.#dependencies.producer.produce({
      workflowStep: this.#route.workflowStep,
      ordinal: this.#route.ordinal,
      capabilityId: this.#route.capabilityId,
      arguments: argumentsValue
    }, context);
    signal.throwIfAborted();
    if (produced.taskId !== context.taskId || produced.runId !== context.runId || !sha256Schema.safeParse(produced.invocationId).success) {
      throw new Error("Temporal MCP dispatch producer returned a conflicting binding");
    }
    return produced;
  }

  private async projectResult(
    result: McpWorkerDispatchResult,
    context: ExecutionContext,
    produced: McpInvocationProducerResult,
    argumentsValue: Readonly<Record<string, unknown>>,
    input: Extract<TemporalMcpDispatchActivityInput, { kind: "begin" }>,
    signal: AbortSignal
  ): Promise<AgentTaskDirective> {
    if (result.kind === "complete") return this.projectComplete(result, context, produced, signal);
    const checkpoint = createCheckpoint(this.#route, produced, argumentsValue, result.checkpoint, input);
    return {
      ...result.directive,
      checkpoint
    };
  }

  private async projectComplete(
    result: Extract<McpWorkerDispatchResult, { kind: "complete" }>,
    context: ExecutionContext,
    produced: McpInvocationProducerResult,
    signal: AbortSignal
  ): Promise<Extract<AgentTaskDirective, { kind: "complete" }>> {
    const projected = artifactProjectionSchema.parse(await this.#dependencies.projector.project({
      context,
      invocationId: produced.invocationId,
      roundId: result.receipt.roundId,
      result: result.result
    }, signal));
    signal.throwIfAborted();
    return {
      kind: "complete",
      output: {
        invocationId: produced.invocationId,
        roundId: result.receipt.roundId,
        artifactId: projected.artifactId,
        artifactVersion: projected.artifactVersion
      }
    };
  }
}

function parseInput(rawInput: unknown): TemporalMcpDispatchActivityInput {
  const begin = beginInputSchema.safeParse(rawInput);
  if (begin.success) {
    return {
      kind: "begin",
      routeId: begin.data.routeId,
      routeVersion: begin.data.routeVersion,
      taskId: begin.data.taskId,
      runId: begin.data.runId,
      principalUserId: begin.data.principalUserId,
      arguments: begin.data.arguments,
      ...(begin.data.requestId === undefined ? {} : { requestId: begin.data.requestId }),
      ...(begin.data.traceId === undefined ? {} : { traceId: begin.data.traceId })
    };
  }
  const resume = resumeInputSchema.safeParse(rawInput);
  if (!resume.success) throw new Error("Temporal MCP dispatch input is invalid");
  return resume.data as TemporalMcpDispatchActivityInput;
}

function canonicalArguments(raw: unknown): Readonly<Record<string, unknown>> {
  if (!isRecord(raw)) throw new Error("Temporal MCP dispatch arguments must be an object");
  const canonical = canonicalMcpJSON(raw);
  if (Buffer.byteLength(canonical, "utf8") > 16 * 1024) {
    throw new Error("Temporal MCP dispatch arguments exceed 16 KiB");
  }
  return JSON.parse(canonical) as Readonly<Record<string, unknown>>;
}

function createCheckpoint(
  route: TemporalMcpDispatchRoute,
  produced: McpInvocationProducerResult,
  argumentsValue: Readonly<Record<string, unknown>>,
  workerCheckpoint: unknown,
  input: Extract<TemporalMcpDispatchActivityInput, { kind: "begin" }>
): TemporalMcpDispatchCheckpointV1 {
  const binding = {
    schemaVersion: checkpointSchemaVersion,
    taskId: produced.taskId,
    runId: produced.runId,
    principalUserId: input.principalUserId,
    routeId: route.routeId,
    routeVersion: route.routeVersion,
    capabilityId: route.capabilityId,
    workflowStep: route.workflowStep,
    ordinal: route.ordinal,
    invocationId: produced.invocationId,
    arguments: argumentsValue,
    workerCheckpoint,
    ...(input.requestId === undefined ? {} : { requestId: input.requestId }),
    ...(input.traceId === undefined ? {} : { traceId: input.traceId })
  };
  return { ...binding, bindingSha256: sha256(canonicalMcpJSON(binding)) };
}

function parseCheckpoint(raw: unknown, route: TemporalMcpDispatchRoute): TemporalMcpDispatchCheckpointV1 {
  const parsed = checkpointSchema.safeParse(raw);
  if (!parsed.success) throw new Error("Temporal MCP dispatch checkpoint is invalid");
  const { bindingSha256, ...binding } = parsed.data;
  if (bindingSha256 !== sha256(canonicalMcpJSON(binding)) || parsed.data.routeId !== route.routeId ||
      parsed.data.routeVersion !== route.routeVersion || parsed.data.capabilityId !== route.capabilityId ||
      parsed.data.workflowStep !== route.workflowStep || parsed.data.ordinal !== route.ordinal) {
    throw new Error("Temporal MCP dispatch checkpoint binding is invalid");
  }
  return parsed.data as TemporalMcpDispatchCheckpointV1;
}

function toElicitationResult(resume: AgentTaskResume): McpElicitationResultInput {
  if (!isRecord(resume) || resume.kind !== "input") {
    throw new Error("Temporal MCP dispatch resume requires durable input");
  }
  return { action: "accept", resume };
}

function correlation(value: { readonly requestId?: string; readonly traceId?: string }) {
  return {
    ...(value.requestId === undefined ? {} : { requestId: value.requestId }),
    ...(value.traceId === undefined ? {} : { traceId: value.traceId })
  };
}

function temporalCancellationSignal(): AbortSignal {
  return TemporalActivityContext.current().cancellationSignal;
}

function sha256(value: string): string {
  return createHash("sha256").update(value, "utf8").digest("hex");
}

function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
