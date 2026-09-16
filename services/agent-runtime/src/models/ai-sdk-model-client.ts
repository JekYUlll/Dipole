import { extractJsonMiddleware, generateText, Output, stepCountIs, wrapLanguageModel } from "ai";
import { createOpenAI } from "@ai-sdk/openai";
import type { z } from "zod";

import type { StructuredModelClient } from "./model-router.js";

type WrappableLanguageModel = Parameters<typeof wrapLanguageModel>[0]["model"];

export class AISDKStructuredModelClient implements StructuredModelClient {
  constructor(private readonly resolveModel: (route: string) => WrappableLanguageModel = defaultModelResolver) {}

  async generate(input: Parameters<StructuredModelClient["generate"]>[0]): ReturnType<StructuredModelClient["generate"]> {
    const request = {
      // Compatible providers occasionally wrap JSON in Markdown fences. Keep
      // that tolerance inside the AI SDK output pipeline.
      model: wrapLanguageModel({ model: this.resolveModel(input.route), middleware: extractJsonMiddleware() }),
      ...(input.system === undefined ? {} : { system: input.system }),
      prompt: input.prompt,
      ...(input.tools === undefined ? {} : { tools: input.tools, activeTools: input.activeTools, stopWhen: stepCountIs(8) }),
      maxRetries: 0,
      maxOutputTokens: input.maxOutputTokens,
      timeout: input.timeoutMs,
      providerOptions: {
        // The configured compatible model otherwise spends the full output
        // budget on hidden reasoning before returning the structured plan.
        openai: { reasoningEffort: "none" }
      }
    };
    try {
      const result = await generateText({ ...request, output: Output.object({ schema: input.schema }) });
      return structuredResult(input, result.output, result);
    } catch (error) {
      if (!responseFormatUnavailable(error)) throw error;
    }

    const fallback = await generateText({
      ...request,
      system: strictJSONInstruction(input.system, input.schema),
      prompt: `${input.prompt}\n\nReturn only the JSON object described by the system instruction.`
    });
    return structuredResult(input, parseJSONObject(fallback.text), fallback);
  }
}

function structuredResult(
  input: Parameters<StructuredModelClient["generate"]>[0],
  output: unknown,
  result: { readonly usage: { readonly inputTokens: number | undefined; readonly outputTokens: number | undefined }; readonly finishReason: string }
): Awaited<ReturnType<StructuredModelClient["generate"]>> {
  const parsed = input.schema.parse(output);
  return {
    output: parsed,
    usage: {
      inputTokens: result.usage.inputTokens,
      outputTokens: result.usage.outputTokens
    },
    finishReason: result.finishReason
  };
}

function responseFormatUnavailable(error: unknown): boolean {
  return /response_format.*unavailable/i.test(error instanceof Error ? error.message : String(error));
}

function strictJSONInstruction(system: string | undefined, schema: z.ZodType): string {
  return `${system ?? ""}\nReturn exactly one JSON object with no Markdown or extra text. Validate it against this JSON Schema: ${JSON.stringify(schema.toJSONSchema())}`;
}

function parseJSONObject(text: string): unknown {
  const candidate = text.trim().replace(/^```(?:json)?\s*/i, "").replace(/\s*```$/, "");
  return JSON.parse(candidate);
}

function defaultModelResolver(route: string): WrappableLanguageModel {
  const apiKey = process.env.DIPOLE_AGENT_MODEL_API_KEY ?? process.env.DEEPSEEK_API_KEY ?? process.env.DIPOLE_AI_API_KEY ?? "";
  if (!apiKey.trim()) {
    throw new Error("Agent model API key is required");
  }
  const baseURL = process.env.DIPOLE_AGENT_MODEL_BASE_URL ?? process.env.DEEPSEEK_BASE_URL ?? "https://api.deepseek.com/v1";
  return createOpenAI({ apiKey, baseURL }).chat(route.trim());
}
