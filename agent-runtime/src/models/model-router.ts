import type { z } from "zod";

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
  generate(input: StructuredModelRequest): Promise<{ readonly output: unknown; readonly usage: ModelUsage }>;
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
    private readonly now: () => number = () => Date.now()
  ) {
    this.#routes = normalizeRoutes(routes);
    this.#policy = validatePolicy(policy);
  }

  async generate<T>(input: { readonly prompt: string; readonly schema: z.ZodType<T> }): Promise<ModelRoutingResult<T>> {
    const startedAt = this.now();
    const errors: unknown[] = [];
    let attempts = 0;

    for (const route of this.#routes) {
      const remainingMs = this.#policy.totalTimeoutMs - (this.now() - startedAt);
      if (attempts >= this.#policy.maxCalls || remainingMs <= 0) {
        break;
      }
      attempts += 1;
      try {
        const response = await this.client.generate({
          route,
          prompt: input.prompt,
          schema: input.schema,
          maxOutputTokens: this.#policy.maxOutputTokensPerCall,
          timeoutMs: Math.max(1, Math.floor(remainingMs))
        });
        return {
          output: input.schema.parse(response.output),
          route,
          attempts,
          usage: response.usage
        };
      } catch (error) {
        errors.push(error);
      }
    }

    const exhaustedBudget = attempts >= this.#policy.maxCalls
      || this.now() - startedAt >= this.#policy.totalTimeoutMs;
    throw new ModelRoutingError(attempts, exhaustedBudget, errors);
  }
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
