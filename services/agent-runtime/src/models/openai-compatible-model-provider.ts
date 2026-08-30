import { createOpenAICompatible } from "@ai-sdk/openai-compatible";
import type { LanguageModel } from "ai";
import { z } from "zod";

const providerNameSchema = z.string().trim().regex(/^[a-z][a-z0-9_-]{0,63}$/);

export const modelProviderConfigSchema = z.object({
  kind: z.enum(["disabled", "openai_compatible"]),
  name: z.string().trim(),
  baseURL: z.string().trim(),
  apiKey: z.string()
}).strict().superRefine((config, refinement) => {
  if (config.kind === "disabled") return;
  if (!providerNameSchema.safeParse(config.name).success) {
    refinement.addIssue({ code: "custom", message: "Model provider name must be a lowercase identifier", path: ["name"] });
  }
  if (config.apiKey.trim().length === 0) {
    refinement.addIssue({ code: "custom", message: "OpenAI-compatible model provider requires an API key", path: ["apiKey"] });
  }
  try {
    const url = new URL(config.baseURL);
    if ((url.protocol !== "https:" && !(url.protocol === "http:" && isLoopbackHost(url.hostname))) || url.username || url.password || url.search || url.hash) {
      throw new Error("invalid");
    }
  } catch {
    refinement.addIssue({
      code: "custom",
      message: "OpenAI-compatible model provider base URL must use HTTPS or loopback HTTP without credentials, query, or fragment",
      path: ["baseURL"]
    });
  }
});

export type ModelProviderConfig = z.infer<typeof modelProviderConfigSchema>;

export function loadModelProviderConfig(env: NodeJS.ProcessEnv): ModelProviderConfig {
  return modelProviderConfigSchema.parse({
    kind: env.DIPOLE_AGENT_MODEL_PROVIDER?.trim().toLowerCase() || "disabled",
    name: env.DIPOLE_AGENT_MODEL_PROVIDER_NAME ?? "",
    baseURL: env.DIPOLE_AGENT_MODEL_BASE_URL ?? "",
    apiKey: env.DIPOLE_AGENT_MODEL_API_KEY ?? ""
  });
}

export function createOpenAICompatibleModelResolver(config: ModelProviderConfig): (route: string) => LanguageModel {
  if (config.kind !== "openai_compatible") {
    throw new Error("AI SDK model mode requires an enabled OpenAI-compatible model provider");
  }
  const provider = createOpenAICompatible({
    name: config.name,
    baseURL: config.baseURL,
    apiKey: config.apiKey
  });
  return (route) => provider(modelIDForRoute(route, config.name));
}

export function modelIDForRoute(route: string, providerName: string): string {
  const prefix = `${providerName}/`;
  if (!route.startsWith(prefix) || route.length === prefix.length) {
    throw new Error(`Model route must use ${prefix}<model-id>`);
  }
  return route.slice(prefix.length);
}

function isLoopbackHost(host: string): boolean {
  return host === "localhost" || host === "::1" || host.startsWith("127.");
}
