import { generateText, Output, type JSONValue, type LanguageModel } from "ai";
import { z } from "zod";

import type { StructuredModelClient } from "./model-router.js";

export class AISDKStructuredModelClient implements StructuredModelClient {
  constructor(
    private readonly resolveModel: (route: string) => LanguageModel,
    private readonly outputMode: "json_schema" | "json_text" = "json_schema",
    private readonly providerOptions?: Record<string, Record<string, JSONValue>>
  ) {}

  async generate(input: Parameters<StructuredModelClient["generate"]>[0]): ReturnType<StructuredModelClient["generate"]> {
    const result = await generateText({
      model: this.resolveModel(input.route),
      prompt: this.outputMode === "json_text" ? jsonTextPrompt(input.prompt, input.schema) : input.prompt,
      ...(this.outputMode === "json_schema" ? { output: Output.object({ schema: input.schema }) } : {}),
      ...(this.providerOptions === undefined ? {} : { providerOptions: this.providerOptions }),
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
  const trimmed = text.trim().replace(/^<think>[\s\S]*?<\/think>\s*/i, "");
  const fenced = /^```(?:json)?\s*\n([\s\S]*?)\n```$/i.exec(trimmed);
  try {
    return JSON.parse(extractTerminalJSONObject(fenced?.[1] ?? trimmed));
  } catch {
    throw new Error(`model JSON-text response is not a valid JSON object (${jsonTextShape(trimmed)})`);
  }
}

function jsonTextShape(text: string): string {
  const candidate = text.trim();
  const first = candidate[0] ?? "empty";
  return [
    `bytes=${Buffer.byteLength(candidate, "utf8")}`,
    `first=${JSON.stringify(first)}`,
    `has_object=${candidate.includes("{")}`,
    `has_think_open=${candidate.includes("<think>")}`,
    `has_think_close=${candidate.includes("</think>")}`,
    `has_fence=${candidate.includes("```")}`
  ].join(",");
}

function extractTerminalJSONObject(text: string): string {
  const trimmed = text.trim();
  const start = trimmed.indexOf("{");
  if (start < 0) return trimmed;
  const prefix = trimmed.slice(0, start).trim();
  if (prefix.includes("}") || Buffer.byteLength(prefix, "utf8") > 1024) return trimmed;
  let depth = 0;
  let quoted = false;
  let escaped = false;
  for (let index = start; index < trimmed.length; index += 1) {
    const character = trimmed[index]!;
    if (quoted) {
      if (escaped) escaped = false;
      else if (character === "\\") escaped = true;
      else if (character === '"') quoted = false;
      continue;
    }
    if (character === '"') quoted = true;
    else if (character === "{") depth += 1;
    else if (character === "}") {
      depth -= 1;
      if (depth === 0) {
        const suffix = trimmed.slice(index + 1).trim();
        // Some OpenAI-compatible providers append a short explanation after a
        // valid JSON response. Only recover the one bounded object; a second
        // object remains ambiguous and is rejected by the normal parse path.
        if (!suffix.includes("{") && !suffix.includes("}") && Buffer.byteLength(suffix, "utf8") <= 1024) {
          return trimmed.slice(start, index + 1);
        }
        return trimmed;
      }
    }
  }
  return trimmed;
}
