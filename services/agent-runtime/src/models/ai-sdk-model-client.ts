import { extractJsonMiddleware, generateText, Output, wrapLanguageModel } from "ai";
import { createOpenAI } from "@ai-sdk/openai";

import type { StructuredModelClient } from "./model-router.js";

type WrappableLanguageModel = Parameters<typeof wrapLanguageModel>[0]["model"];

export class AISDKStructuredModelClient implements StructuredModelClient {
  constructor(private readonly resolveModel: (route: string) => WrappableLanguageModel = defaultModelResolver) {}

  async generate(input: Parameters<StructuredModelClient["generate"]>[0]): ReturnType<StructuredModelClient["generate"]> {
    const result = await generateText({
      // Compatible providers occasionally wrap JSON in Markdown fences. Keep
      // that tolerance inside the AI SDK output pipeline.
      model: wrapLanguageModel({ model: this.resolveModel(input.route), middleware: extractJsonMiddleware() }),
      ...(input.system === undefined ? {} : { system: input.system }),
      prompt: input.prompt,
      output: Output.object({ schema: input.schema }),
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
      output: result.output,
      usage: {
        inputTokens: result.usage.inputTokens,
        outputTokens: result.usage.outputTokens
      },
      finishReason: result.finishReason
    };
  }
}

function defaultModelResolver(route: string): WrappableLanguageModel {
  const apiKey = process.env.DIPOLE_AGENT_MODEL_API_KEY ?? process.env.DEEPSEEK_API_KEY ?? process.env.DIPOLE_AI_API_KEY ?? "";
  if (!apiKey.trim()) {
    throw new Error("Agent model API key is required");
  }
  const baseURL = process.env.DIPOLE_AGENT_MODEL_BASE_URL ?? process.env.DEEPSEEK_BASE_URL ?? "https://api.deepseek.com/v1";
  return createOpenAI({ apiKey, baseURL }).chat(route.trim());
}
