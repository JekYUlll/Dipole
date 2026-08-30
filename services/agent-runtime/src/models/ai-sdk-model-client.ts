import { generateText, Output, type LanguageModel } from "ai";

import type { StructuredModelClient } from "./model-router.js";

export class AISDKStructuredModelClient implements StructuredModelClient {
  constructor(private readonly resolveModel: (route: string) => LanguageModel) {}

  async generate(input: Parameters<StructuredModelClient["generate"]>[0]): ReturnType<StructuredModelClient["generate"]> {
    const result = await generateText({
      model: this.resolveModel(input.route),
      prompt: input.prompt,
      output: Output.object({ schema: input.schema }),
      maxRetries: 0,
      maxOutputTokens: input.maxOutputTokens,
      timeout: input.timeoutMs
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
