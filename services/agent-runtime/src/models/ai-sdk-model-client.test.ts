import { describe, expect, it, vi } from "vitest";
import { tool } from "ai";
import { z } from "zod";
import { MockLanguageModelV3 } from "ai/test";

import { AISDKStructuredModelClient } from "./ai-sdk-model-client.js";

describe("AISDKStructuredModelClient", () => {
  it("uses AI SDK native structured output without hidden retries", async () => {
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
      usage: { inputTokens: 21, outputTokens: 7 },
      finishReason: "stop"
    });
    expect(model.doGenerateCalls).toHaveLength(1);
    expect(model.doGenerateCalls[0]).toMatchObject({
      maxOutputTokens: 96,
      responseFormat: {
        type: "json",
      },
      providerOptions: { openai: { reasoningEffort: "none" } }
    });
  });

  it("validates generated output with the caller schema", async () => {
    const model = new MockLanguageModelV3({
      provider: "test",
      modelId: "planner",
      doGenerate: {
        content: [{ type: "text", text: "```json\n{\"summary\":\"ready\"}\n```" }],
        finishReason: { unified: "stop", raw: "stop" },
        usage: { inputTokens: { total: 1, noCache: 1, cacheRead: 0, cacheWrite: 0 }, outputTokens: { total: 1, text: 1, reasoning: 0 } },
        warnings: []
      }
    });
    const client = new AISDKStructuredModelClient(() => model);

    await expect(client.generate({
      route: "test/planner", prompt: "plan event", schema: z.object({ summary: z.string() }), maxOutputTokens: 96, timeoutMs: 2000
    })).resolves.toMatchObject({ output: { summary: "ready" } });
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

  it("falls back to locally validated JSON when a provider rejects response_format", async () => {
    const doGenerate = vi.fn(async (options: { responseFormat?: { type?: string } }) => {
      if (options.responseFormat?.type === "json") {
        throw new Error("This response_format type is unavailable now");
      }
      return {
        content: [{ type: "text" as const, text: '{"summary":"ready"}' }],
        finishReason: { unified: "stop" as const, raw: "stop" },
        usage: { inputTokens: { total: 2, noCache: 2, cacheRead: 0, cacheWrite: 0 }, outputTokens: { total: 1, text: 1, reasoning: 0 } },
        warnings: []
      };
    });
    const model = new MockLanguageModelV3({ provider: "test", modelId: "fallback", doGenerate });
    const client = new AISDKStructuredModelClient(() => model);

    await expect(client.generate({
      route: "test/fallback", prompt: "plan event", schema: z.object({ summary: z.string() }), maxOutputTokens: 96, timeoutMs: 2000
    })).resolves.toMatchObject({ output: { summary: "ready" } });
    expect(doGenerate).toHaveBeenCalledTimes(2);
    expect(model.doGenerateCalls[1]?.responseFormat).not.toMatchObject({ type: "json" });
  });

  it("forwards active read tools to the AI SDK tool loop", async () => {
    const model = new MockLanguageModelV3({
      provider: "test", modelId: "planner",
      doGenerate: {
        content: [{ type: "text", text: JSON.stringify({ summary: "北京时间是 12:34" }) }],
        finishReason: { unified: "stop", raw: "stop" },
        usage: { inputTokens: { total: 1, noCache: 1, cacheRead: 0, cacheWrite: 0 }, outputTokens: { total: 1, text: 1, reasoning: 0 } },
        warnings: []
      }
    });
    const client = new AISDKStructuredModelClient(() => model);

    await client.generate({
      route: "test/planner", prompt: "现在几点", schema: z.object({ summary: z.string() }), maxOutputTokens: 96, timeoutMs: 2000,
      tools: { get_current_time: tool({ inputSchema: z.object({}), execute: async () => ({ time: "12:34" }) }) },
      activeTools: ["get_current_time"]
    });

    expect(model.doGenerateCalls[0]).toMatchObject({
      tools: [expect.objectContaining({ name: "get_current_time" })]
    });
  });
});
