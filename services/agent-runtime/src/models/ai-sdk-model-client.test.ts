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
      usage: { inputTokens: 21, outputTokens: 7 },
      finishReason: "stop"
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

  it("validates one JSON-text response locally when the provider lacks JSON Schema response format", async () => {
    const model = new MockLanguageModelV3({
      provider: "test",
      modelId: "json-text",
      doGenerate: {
        content: [{ type: "text", text: JSON.stringify({ summary: "observe E2", capabilityIds: ["conversation.read"] }) }],
        finishReason: { unified: "stop", raw: "stop" },
        usage: {
          inputTokens: { total: 18, noCache: 18, cacheRead: 0, cacheWrite: 0 },
          outputTokens: { total: 9, text: 9, reasoning: 0 }
        },
        warnings: []
      }
    });
    const client = new AISDKStructuredModelClient(() => model, "json_text");
    const schema = z.object({ summary: z.string(), capabilityIds: z.array(z.string()) });

    await expect(client.generate({
      route: "test/json-text", prompt: "plan event", schema, maxOutputTokens: 96, timeoutMs: 2000
    })).resolves.toMatchObject({ output: { summary: "observe E2", capabilityIds: ["conversation.read"] } });
    expect(model.doGenerateCalls).toHaveLength(1);
    expect(model.doGenerateCalls[0]?.prompt).toEqual(expect.arrayContaining([
      expect.objectContaining({ content: expect.stringContaining("Return only a single JSON object") })
    ]));
  });
});
