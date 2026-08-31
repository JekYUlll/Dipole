import { describe, expect, it } from "vitest";

import {
  createOpenAICompatibleModelResolver,
  loadModelProviderConfig,
  modelIDForRoute
} from "./openai-compatible-model-provider.js";

describe("OpenAI-compatible model provider", () => {
  const environment = {
    DIPOLE_AGENT_MODEL_PROVIDER: "openai_compatible",
    DIPOLE_AGENT_MODEL_PROVIDER_NAME: "gateway",
    DIPOLE_AGENT_MODEL_BASE_URL: "https://models.example.test/v1",
    DIPOLE_AGENT_MODEL_API_KEY: "test-model-key"
  };

  it("creates a provider-scoped resolver without exposing the API key", () => {
    const config = loadModelProviderConfig({
      ...environment,
      DIPOLE_AGENT_MODEL_STRUCTURED_OUTPUTS: "true"
    });
    const resolve = createOpenAICompatibleModelResolver(config);

    expect(resolve("gateway/gpt-5-mini")).toMatchObject({ supportsStructuredOutputs: true });
    expect(modelIDForRoute("gateway/gpt-5-mini", "gateway")).toBe("gpt-5-mini");
  });

  it("keeps structured output disabled until the provider is explicitly declared compatible", () => {
    const config = loadModelProviderConfig(environment);

    expect(createOpenAICompatibleModelResolver(config)("gateway/gpt-5-mini")).toMatchObject({ supportsStructuredOutputs: false });
  });

  it("rejects malformed provider configuration and cross-provider routes", () => {
    expect(() => loadModelProviderConfig({
      ...environment, DIPOLE_AGENT_MODEL_BASE_URL: "https://user:secret@models.example.test/v1"
    })).toThrow(/HTTPS or loopback HTTP/);
    expect(() => loadModelProviderConfig({
      ...environment, DIPOLE_AGENT_MODEL_BASE_URL: "http://models.example.test/v1"
    })).toThrow(/HTTPS or loopback HTTP/);
    expect(() => loadModelProviderConfig({ ...environment, DIPOLE_AGENT_MODEL_API_KEY: "" })).toThrow(/API key/);
    expect(() => modelIDForRoute("other/gpt-5-mini", "gateway")).toThrow(/gateway/);
  });
});
