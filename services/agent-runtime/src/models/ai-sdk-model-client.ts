import { generateText, type LanguageModel } from "ai";
import { createOpenAI } from "@ai-sdk/openai";
import { z } from "zod";

import type { StructuredModelClient } from "./model-router.js";

export class AISDKStructuredModelClient implements StructuredModelClient {
  constructor(private readonly resolveModel: (route: string) => LanguageModel = defaultModelResolver) {}

  async generate(input: Parameters<StructuredModelClient["generate"]>[0]): ReturnType<StructuredModelClient["generate"]> {
    const result = await generateText({
      model: this.resolveModel(input.route),
      prompt: `${input.prompt}\n\nReturn only a JSON object matching this schema:\n${JSON.stringify(z.toJSONSchema(input.schema))}`,
      maxRetries: 0,
      maxOutputTokens: input.maxOutputTokens,
      timeout: input.timeoutMs,
      providerOptions: {
        // The configured compatible model otherwise spends the full output
        // budget on hidden reasoning before returning the structured plan.
        openai: { reasoningEffort: "none" }
      }
    });
    return {
      output: input.schema.parse(parseJSONObject(result.text)),
      usage: {
        inputTokens: result.usage.inputTokens,
        outputTokens: result.usage.outputTokens
      },
      finishReason: result.finishReason
    };
  }
}

function parseJSONObject(text: string): unknown {
  const normalized = text.trim().replace(/^```(?:json)?\s*/i, "").replace(/\s*```$/, "");
  return JSON.parse(normalized);
}

function defaultModelResolver(route: string): LanguageModel {
  const apiKey = process.env.DIPOLE_AGENT_MODEL_API_KEY ?? process.env.DEEPSEEK_API_KEY ?? process.env.DIPOLE_AI_API_KEY ?? "";
  if (!apiKey.trim()) {
    throw new Error("Agent model API key is required");
  }
  const baseURL = process.env.DIPOLE_AGENT_MODEL_BASE_URL ?? process.env.DEEPSEEK_BASE_URL ?? "https://api.deepseek.com/v1";
  return createOpenAI({ apiKey, baseURL }).chat(route.trim());
}
