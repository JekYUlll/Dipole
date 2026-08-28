import type { z } from "zod";
import { AgentTelemetry } from "../observability/agent-telemetry.js";

export interface ModelUsage {
  readonly inputTokens: number | undefined;
  readonly outputTokens: number | undefined;
}

export interface StructuredModelRequest {
  readonly route: string;
  readonly prompt: string;
  readonly schema: z.ZodType;
  readonly maxOutputTokens: number;
  readonly timeoutMs: number;
}

export interface StructuredModelClient {
  generate(input: StructuredModelRequest): Promise<{
    readonly output: unknown;
    readonly usage: ModelUsage;
    readonly finishReason?: string;
  }>;
}

export interface ModelRunBudgetPolicy {
  readonly maxCalls: number;
  readonly totalTimeoutMs: number;
  readonly maxOutputTokensPerCall: number;
}

export interface ModelRoutingResult<T> {
  readonly output: T;
  readonly route: string;
  readonly attempts: number;
  readonly usage: ModelUsage;
}

export interface ModelCallReservation {
  readonly runId: string;
  readonly callId: string;
  readonly callNo: number;
  readonly route: string;
}

export interface ModelCallRecovery extends ModelCallReservation {
  readonly output: unknown;
  readonly usage: ModelUsage;
}

export interface ModelAuditStore {
  recover(taskId: string, policy: ModelRunBudgetPolicy): Promise<ModelCallRecovery | undefined>;
  reserve(taskId: string, policy: ModelRunBudgetPolicy, route: string): Promise<ModelCallReservation | undefined>;
  completeCall(reservation: ModelCallReservation, output: unknown, usage: ModelUsage, finishReason: string, latencyMs: number): Promise<void>;
  failCall(reservation: ModelCallReservation, error: unknown, latencyMs: number): Promise<void>;
  completeRun(runId: string): Promise<void>;
  failRun(runId: string, error: unknown): Promise<void>;
  failTask(taskId: string, error: unknown): Promise<void>;
}

export interface ModelTimelineSink {
  appendAgentTaskTimelineEvent(input: {
    readonly eventId: string; readonly taskId: string; readonly runId: string; readonly kind: string; readonly status: string;
    readonly capabilityId?: string; readonly occurredAtUnixMs: number;
  }): Promise<unknown>;
}

export class ModelRoutingError extends Error {
  constructor(
    readonly attempts: number,
    readonly exhaustedBudget: boolean,
    readonly causes: readonly unknown[]
  ) {
    super(exhaustedBudget ? "model run budget exhausted" : "all model routes failed");
    this.name = "ModelRoutingError";
  }
}

export class ModelRouter {
  readonly #routes: readonly string[];
  readonly #policy: ModelRunBudgetPolicy;

  constructor(
    private readonly client: StructuredModelClient,
    routes: readonly string[],
    policy: ModelRunBudgetPolicy,
    private readonly now: () => number = () => Date.now(),
    private readonly audit?: ModelAuditStore,
    private readonly telemetry: Pick<AgentTelemetry, "withSpan"> = new AgentTelemetry(),
    private readonly timeline?: ModelTimelineSink
  ) {
    this.#routes = normalizeRoutes(routes);
    this.#policy = validatePolicy(policy);
  }

  async generate<T>(input: {
    readonly prompt: string;
    readonly schema: z.ZodType<T>;
    readonly taskId?: string;
  }): Promise<ModelRoutingResult<T>> {
    if (this.audit !== undefined && !input.taskId?.trim()) {
      throw new Error("persistent model routing requires a Task ID");
    }
    if (this.audit !== undefined) {
      const recovered = await this.audit.recover(input.taskId!, this.#policy);
      if (recovered !== undefined) {
        const output = input.schema.parse(recovered.output);
        await this.audit.completeRun(recovered.runId);
        return {
          output,
          route: recovered.route,
          attempts: recovered.callNo,
          usage: recovered.usage
        };
      }
    }
    const startedAt = this.now();
    const errors: unknown[] = [];
    let attempts = 0;
    let runId: string | undefined;

    for (const route of this.#routes) {
      const remainingMs = this.#policy.totalTimeoutMs - (this.now() - startedAt);
      if (attempts >= this.#policy.maxCalls || remainingMs <= 0) {
        break;
      }
      const reservation = this.audit === undefined
        ? undefined
        : await this.audit.reserve(input.taskId!, this.#policy, route);
      if (this.audit !== undefined && reservation === undefined) {
        break;
      }
      runId = reservation?.runId ?? runId;
      attempts += 1;
      if (reservation !== undefined) {
        await this.appendTimeline({
          eventId: `model:${reservation.callId}:begin`, taskId: input.taskId!, runId: reservation.runId,
          kind: "model_call", status: "running", occurredAtUnixMs: this.now()
        });
      }
      const callStartedAt = this.now();
      let response: Awaited<ReturnType<StructuredModelClient["generate"]>>;
      try {
        const generate = async (span?: Parameters<Parameters<typeof this.telemetry.withSpan>[2]>[0]) => {
          const value = await this.client.generate({
            route, prompt: input.prompt, schema: input.schema,
            maxOutputTokens: this.#policy.maxOutputTokensPerCall,
            timeoutMs: Math.max(1, Math.floor(remainingMs))
          });
          const validated = { ...value, output: input.schema.parse(value.output) };
          if (span !== undefined) {
            if (value.usage.inputTokens !== undefined) span.setAttribute("dipole.agent.model.input_tokens", value.usage.inputTokens);
            if (value.usage.outputTokens !== undefined) span.setAttribute("dipole.agent.model.output_tokens", value.usage.outputTokens);
          }
          return validated;
        };
        response = input.taskId === undefined ? await generate() : await this.telemetry.withSpan("agent.model.call", {
          taskId: input.taskId,
          ...(reservation?.runId === undefined ? {} : { runId: reservation.runId }),
          attributes: { "dipole.agent.model.route": route, "dipole.agent.model.attempt": attempts }
        }, generate);
      } catch (error) {
        if (reservation !== undefined) {
          await this.audit!.failCall(reservation, error, elapsed(this.now(), callStartedAt));
          await this.appendTimeline({
            eventId: `model:${reservation.callId}:finish`, taskId: input.taskId!, runId: reservation.runId,
            kind: "model_call", status: "failed", occurredAtUnixMs: this.now()
          });
        }
        errors.push(error);
        continue;
      }
      if (reservation !== undefined) {
        await this.audit!.completeCall(
          reservation, response.output, response.usage, response.finishReason ?? "unknown", elapsed(this.now(), callStartedAt)
        );
        await this.audit!.completeRun(reservation.runId);
        await this.appendTimeline({
          eventId: `model:${reservation.callId}:finish`, taskId: input.taskId!, runId: reservation.runId,
          kind: "model_call", status: "completed", occurredAtUnixMs: this.now()
        });
      }
      return {
        output: response.output as T,
        route,
        attempts,
        usage: response.usage
      };
    }

    const exhaustedBudget = attempts >= this.#policy.maxCalls
      || this.now() - startedAt >= this.#policy.totalTimeoutMs;
    const failure = new ModelRoutingError(attempts, exhaustedBudget || this.audit !== undefined, errors);
    if (runId !== undefined) {
      await this.audit!.failRun(runId, failure);
    } else if (this.audit !== undefined) {
      await this.audit.failTask(input.taskId!, failure);
    }
    throw failure;
  }

  private async appendTimeline(input: Parameters<ModelTimelineSink["appendAgentTaskTimelineEvent"]>[0]): Promise<void> {
    if (this.timeline === undefined) return;
    try { await this.timeline.appendAgentTaskTimelineEvent(input); } catch { /* Timeline is a secondary projection. */ }
  }
}

function elapsed(now: number, startedAt: number): number {
  return Math.max(0, Math.min(4_294_967_295, Math.floor(now - startedAt)));
}

function normalizeRoutes(routes: readonly string[]): readonly string[] {
  const normalized = routes.map((route) => route.trim());
  if (normalized.length === 0 || normalized.some((route) => !route)) {
    throw new Error("at least one non-empty model route is required");
  }
  if (new Set(normalized).size !== normalized.length) {
    throw new Error("model routes must be unique");
  }
  return normalized;
}

function validatePolicy(policy: ModelRunBudgetPolicy): ModelRunBudgetPolicy {
  for (const [name, value] of Object.entries(policy)) {
    if (!Number.isSafeInteger(value) || value < 1) {
      throw new Error(`model budget ${name} must be a positive integer`);
    }
  }
  return { ...policy };
}
