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

  it("passes explicitly configured provider options through to the model call", async () => {
    const model = new MockLanguageModelV3({
      provider: "deepseek", modelId: "deepseek-v4-flash",
      doGenerate: {
        content: [{ type: "text", text: JSON.stringify({ summary: "observe E0", capabilityIds: [] }) }],
        finishReason: { unified: "stop", raw: "stop" },
        usage: { inputTokens: { total: 10, noCache: 10, cacheRead: 0, cacheWrite: 0 }, outputTokens: { total: 5, text: 5, reasoning: 0 } },
        warnings: []
      }
    });
    const client = new AISDKStructuredModelClient(() => model, "json_text", {
      deepseek: { thinking: { type: "disabled" } }
    });

    await client.generate({
      route: "deepseek/deepseek-v4-flash", prompt: "plan event", schema: z.object({ summary: z.string(), capabilityIds: z.array(z.string()) }), maxOutputTokens: 96, timeoutMs: 2000
    });

    expect(model.doGenerateCalls[0]).toMatchObject({ providerOptions: { deepseek: { thinking: { type: "disabled" } } } });
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
    expect(JSON.stringify(model.doGenerateCalls[0]?.prompt)).toContain("Return only a single JSON object");
  });

  it("accepts a complete JSON code fence but still validates the contained object", async () => {
    const model = new MockLanguageModelV3({
      provider: "test", modelId: "json-fence",
      doGenerate: {
        content: [{ type: "text", text: "```json\n{\"summary\":\"observe E3\",\"capabilityIds\":[]}\n```" }],
        finishReason: { unified: "stop", raw: "stop" },
        usage: { inputTokens: { total: 10, noCache: 10, cacheRead: 0, cacheWrite: 0 }, outputTokens: { total: 7, text: 7, reasoning: 0 } }, warnings: []
      }
    });
    const client = new AISDKStructuredModelClient(() => model, "json_text");

    await expect(client.generate({
      route: "test/json-fence", prompt: "plan event", schema: z.object({ summary: z.string(), capabilityIds: z.array(z.string()) }), maxOutputTokens: 96, timeoutMs: 2000
    })).resolves.toMatchObject({ output: { summary: "observe E3", capabilityIds: [] } });
  });

  it("drops a closed reasoning prefix before validating the JSON object", async () => {
    const model = new MockLanguageModelV3({
      provider: "test", modelId: "json-reasoning-prefix",
      doGenerate: {
        content: [{ type: "text", text: "<think>produce a read-only plan</think>\n{\"summary\":\"observe E4\",\"capabilityIds\":[]}" }],
        finishReason: { unified: "stop", raw: "stop" },
        usage: { inputTokens: { total: 10, noCache: 10, cacheRead: 0, cacheWrite: 0 }, outputTokens: { total: 7, text: 7, reasoning: 0 } }, warnings: []
      }
    });
    const client = new AISDKStructuredModelClient(() => model, "json_text");

    await expect(client.generate({
      route: "test/json-reasoning-prefix", prompt: "plan event", schema: z.object({ summary: z.string(), capabilityIds: z.array(z.string()) }), maxOutputTokens: 96, timeoutMs: 2000
    })).resolves.toMatchObject({ output: { summary: "observe E4", capabilityIds: [] } });
  });

  it("accepts a leading label only when one complete JSON object ends the response", async () => {
    const model = new MockLanguageModelV3({
      provider: "test", modelId: "json-leading-label",
      doGenerate: {
        content: [{ type: "text", text: "Plan:\n{\"summary\":\"observe E5\",\"capabilityIds\":[]}" }],
        finishReason: { unified: "stop", raw: "stop" },
        usage: { inputTokens: { total: 10, noCache: 10, cacheRead: 0, cacheWrite: 0 }, outputTokens: { total: 7, text: 7, reasoning: 0 } }, warnings: []
      }
    });
    const client = new AISDKStructuredModelClient(() => model, "json_text");

    await expect(client.generate({
      route: "test/json-leading-label", prompt: "plan event", schema: z.object({ summary: z.string(), capabilityIds: z.array(z.string()) }), maxOutputTokens: 96, timeoutMs: 2000
    })).resolves.toMatchObject({ output: { summary: "observe E5", capabilityIds: [] } });
  });

  it("recovers one bounded JSON object when a provider appends a short explanation", async () => {
    const model = new MockLanguageModelV3({
      provider: "test", modelId: "json-trailing-text",
      doGenerate: {
        content: [{ type: "text", text: "{\"summary\":\"observe E6\",\"capabilityIds\":[]} thanks" }],
        finishReason: { unified: "stop", raw: "stop" },
        usage: { inputTokens: { total: 10, noCache: 10, cacheRead: 0, cacheWrite: 0 }, outputTokens: { total: 7, text: 7, reasoning: 0 } }, warnings: []
      }
    });
    const client = new AISDKStructuredModelClient(() => model, "json_text");

    await expect(client.generate({
      route: "test/json-trailing-text", prompt: "plan event", schema: z.object({ summary: z.string(), capabilityIds: z.array(z.string()) }), maxOutputTokens: 96, timeoutMs: 2000
    })).resolves.toMatchObject({ output: { summary: "observe E6", capabilityIds: [] } });
  });

  it("rejects multiple JSON objects even when the first object validates", async () => {
    const model = new MockLanguageModelV3({
      provider: "test", modelId: "json-multiple-objects",
      doGenerate: {
        content: [{ type: "text", text: "{\"summary\":\"observe E7\",\"capabilityIds\":[]} {\"ignored\":true}" }],
        finishReason: { unified: "stop", raw: "stop" },
        usage: { inputTokens: { total: 10, noCache: 10, cacheRead: 0, cacheWrite: 0 }, outputTokens: { total: 7, text: 7, reasoning: 0 } }, warnings: []
      }
    });
    const client = new AISDKStructuredModelClient(() => model, "json_text");

    await expect(client.generate({
      route: "test/json-multiple-objects", prompt: "plan event", schema: z.object({ summary: z.string(), capabilityIds: z.array(z.string()) }), maxOutputTokens: 96, timeoutMs: 2000
    })).rejects.toThrow(/not a valid JSON object/);
  });
});
