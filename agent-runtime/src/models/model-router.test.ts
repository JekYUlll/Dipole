import { describe, expect, it, vi } from "vitest";
import { z } from "zod";

import { ModelRouter, ModelRoutingError, type StructuredModelClient } from "./model-router.js";

const outputSchema = z.object({ summary: z.string() });

describe("ModelRouter", () => {
  it("uses the primary route and applies the per-call budget", async () => {
    const generate: StructuredModelClient["generate"] = vi.fn(async () => ({
      output: { summary: "primary" }, usage: { inputTokens: 12, outputTokens: 4 }
    }));
    const router = new ModelRouter({ generate }, ["gateway/fast", "gateway/fallback"], {
      maxCalls: 2, totalTimeoutMs: 5000, maxOutputTokensPerCall: 128
    });

    const result = await router.generate({ prompt: "plan event", schema: outputSchema });

    expect(result).toEqual({
      output: { summary: "primary" }, route: "gateway/fast", attempts: 1,
      usage: { inputTokens: 12, outputTokens: 4 }
    });
    expect(generate).toHaveBeenCalledWith(expect.objectContaining({
      route: "gateway/fast", maxOutputTokens: 128, timeoutMs: expect.any(Number)
    }));
  });

  it("falls back in order and charges failed calls to the run", async () => {
    const generate: StructuredModelClient["generate"] = vi.fn()
      .mockRejectedValueOnce(new Error("primary unavailable"))
      .mockResolvedValueOnce({ output: { summary: "fallback" }, usage: { inputTokens: 8, outputTokens: 3 } });
    const router = new ModelRouter({ generate }, ["gateway/primary", "gateway/fallback"], {
      maxCalls: 2, totalTimeoutMs: 5000, maxOutputTokensPerCall: 64
    });

    const result = await router.generate({ prompt: "plan event", schema: outputSchema });

    expect(result.route).toBe("gateway/fallback");
    expect(result.attempts).toBe(2);
    expect(generate).toHaveBeenCalledTimes(2);
  });

  it("falls back when a provider returns schema-incompatible output", async () => {
    const generate: StructuredModelClient["generate"] = vi.fn()
      .mockResolvedValueOnce({ output: { summary: 42 }, usage: { inputTokens: 8, outputTokens: 3 } })
      .mockResolvedValueOnce({ output: { summary: "validated" }, usage: { inputTokens: 9, outputTokens: 4 } });
    const router = new ModelRouter({ generate }, ["primary", "fallback"], {
      maxCalls: 2, totalTimeoutMs: 5000, maxOutputTokensPerCall: 64
    });

    await expect(router.generate({ prompt: "plan event", schema: outputSchema })).resolves.toMatchObject({
      output: { summary: "validated" }, route: "fallback", attempts: 2
    });
  });

  it("stops before an unbudgeted fallback", async () => {
    const generate: StructuredModelClient["generate"] = vi.fn(async () => { throw new Error("unavailable"); });
    const router = new ModelRouter({ generate }, ["one", "two", "three"], {
      maxCalls: 2, totalTimeoutMs: 5000, maxOutputTokensPerCall: 64
    });

    const failure = await router.generate({ prompt: "plan event", schema: outputSchema }).catch((error: unknown) => error);

    expect(failure).toBeInstanceOf(ModelRoutingError);
    expect(failure).toMatchObject({ attempts: 2, exhaustedBudget: true });
    expect(generate).toHaveBeenCalledTimes(2);
  });

  it("does not start another route after the run deadline", async () => {
    let now = 1000;
    const generate: StructuredModelClient["generate"] = vi.fn(async () => {
      now = 1200;
      throw new Error("timeout");
    });
    const router = new ModelRouter({ generate }, ["one", "two"], {
      maxCalls: 2, totalTimeoutMs: 100, maxOutputTokensPerCall: 64
    }, () => now);

    const failure = await router.generate({ prompt: "plan event", schema: outputSchema }).catch((error: unknown) => error);

    expect(failure).toMatchObject({ attempts: 1, exhaustedBudget: true });
    expect(generate).toHaveBeenCalledTimes(1);
    expect(generate).toHaveBeenCalledWith(expect.objectContaining({ timeoutMs: 100 }));
  });
});
