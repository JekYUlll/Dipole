import { describe, expect, it, vi } from "vitest";
import { z } from "zod";
import { MockLanguageModelV3 } from "ai/test";

import { AISDKStructuredModelClient } from "./ai-sdk-model-client.js";

describe("AISDKStructuredModelClient", () => {
  it("uses AI SDK structured output without hidden retries", async () => {
    const model = new MockLanguageModelV3({
      provider: "test",
      modelId: "planner",
      doGenerate: {
        content: [{ type: "text", text: JSON.stringify({ summary: "observe E1", capabilityIds: [] }) }],
        finishReason: { unified: "stop", raw: "stop" },
        usage: {
          inputTokens: { total: 21, noCache: 21, cacheRead: 0, cacheWrite: 0 },
          outputTokens: { total: 7, text: 7, reasoning: 0 }
        },
        warnings: []
      }
    });
    const client = new AISDKStructuredModelClient(() => model);
    const schema = z.object({ summary: z.string(), capabilityIds: z.array(z.string()) });

    const result = await client.generate({
      route: "test/planner", prompt: "plan event", schema, maxOutputTokens: 96, timeoutMs: 2000
    });

    expect(result).toEqual({
      output: { summary: "observe E1", capabilityIds: [] },
      usage: { inputTokens: 21, outputTokens: 7 }
    });
    expect(model.doGenerateCalls).toHaveLength(1);
    expect(model.doGenerateCalls[0]).toMatchObject({ maxOutputTokens: 96 });
  });

  it("does not retry inside AI SDK when the provider fails", async () => {
    const doGenerate = vi.fn(async () => { throw new Error("provider unavailable"); });
    const model = new MockLanguageModelV3({ provider: "test", modelId: "failing", doGenerate });
    const client = new AISDKStructuredModelClient(() => model);

    await expect(client.generate({
      route: "test/failing", prompt: "plan event", schema: z.object({ summary: z.string() }),
      maxOutputTokens: 96, timeoutMs: 2000
    })).rejects.toThrow(/provider unavailable/);
    expect(doGenerate).toHaveBeenCalledOnce();
  });
});
