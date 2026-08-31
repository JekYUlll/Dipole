import { generateText, Output, type LanguageModel } from "ai";
import { z } from "zod";

import type { StructuredModelClient } from "./model-router.js";

export class AISDKStructuredModelClient implements StructuredModelClient {
  constructor(
    private readonly resolveModel: (route: string) => LanguageModel,
    private readonly outputMode: "json_schema" | "json_text" = "json_schema"
  ) {}

  async generate(input: Parameters<StructuredModelClient["generate"]>[0]): ReturnType<StructuredModelClient["generate"]> {
    const result = await generateText({
      model: this.resolveModel(input.route),
      prompt: this.outputMode === "json_text" ? jsonTextPrompt(input.prompt, input.schema) : input.prompt,
      ...(this.outputMode === "json_schema" ? { output: Output.object({ schema: input.schema }) } : {}),
      maxRetries: 0,
      maxOutputTokens: input.maxOutputTokens,
      timeout: input.timeoutMs
    });
    return {
      output: this.outputMode === "json_text" ? input.schema.parse(parseJSONText(result.text)) : result.output,
      usage: {
        inputTokens: result.usage.inputTokens,
        outputTokens: result.usage.outputTokens
      },
      finishReason: result.finishReason
    };
  }
}

function jsonTextPrompt(prompt: string, schema: z.ZodType): string {
  return `${prompt}\n\nReturn only a single JSON object. Do not use Markdown, prose, or code fences. The object must match this JSON Schema:\n${JSON.stringify(z.toJSONSchema(schema))}`;
}

function parseJSONText(text: string): unknown {
  try {
    return JSON.parse(text.trim());
  } catch {
    throw new Error("model JSON-text response is not a valid JSON object");
  }
}
