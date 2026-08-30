import { describe, expect, it, vi } from "vitest";

import { foundationAgentTaskActivities } from "../temporal/agent-task-activities.js";
import { loadTemporalRuntimeConfig } from "../temporal/temporal-runtime.js";
import {
  startExternalMcpProductionShadow,
  validateExternalMcpProductionShadowMode,
  type ExternalMcpProductionShadowSeams
} from "./external-mcp-production-shadow.js";
import { loadShadowRuntimeConfig } from "./shadow-runtime.js";

describe("external MCP production Shadow startup", () => {
  it("keeps disabled configuration free of process side effects", async () => {
    const startProcess = vi.fn<ExternalMcpProductionShadowSeams["startProcess"]>();

    await expect(startExternalMcpProductionShadow(
      {}, shadow(false), temporal(false, false), foundationAgentTaskActivities, {}, undefined, { startProcess }
    )).resolves.toBeUndefined();

    expect(startProcess).not.toHaveBeenCalled();
  });

  it("rejects every partial configuration before process construction", async () => {
    const cases = [
      [enabledEnv(), shadow(false), temporal(true)],
      [enabledEnv(), shadow(true, "direct_target"), temporal(true)],
      [{}, shadow(true), temporal(true)],
      [enabledEnv(), shadow(true), temporal(false)]
    ] as const;

    for (const [env, shadowConfig, temporalConfig] of cases) {
      const startProcess = vi.fn<ExternalMcpProductionShadowSeams["startProcess"]>();

      expect(() => validateExternalMcpProductionShadowMode(env, shadowConfig, temporalConfig))
        .toThrow(/^External MCP production Shadow mode configuration is invalid$/);
      await expect(startExternalMcpProductionShadow(
        env, shadowConfig, temporalConfig, foundationAgentTaskActivities, {}, undefined, { startProcess }
      )).rejects.toThrow(/^External MCP production Shadow mode configuration is invalid$/);
      expect(startProcess).not.toHaveBeenCalled();
    }
  });

  it("hands exact configuration and the sealed route factory to the process owner", async () => {
    const process = { temporal: {} as never, stop: vi.fn(async () => undefined) };
    const startProcess = vi.fn<ExternalMcpProductionShadowSeams["startProcess"]>(async () => process);
    const env = enabledEnv();
    const shadowConfig = shadow(true);
    const temporalConfig = temporal(true);
    const controller = new AbortController();
    const onFailure = vi.fn();

    await expect(startExternalMcpProductionShadow(
      env, shadowConfig, temporalConfig, foundationAgentTaskActivities,
      { signal: controller.signal }, onFailure, { startProcess }
    )).resolves.toBe(process);
    expect(startProcess).toHaveBeenCalledWith(
      env, shadowConfig, temporalConfig, foundationAgentTaskActivities,
      expect.any(Function), { signal: controller.signal }, onFailure
    );
  });

  it("rejects an enabled owner that returns no process", async () => {
    const startProcess = vi.fn<ExternalMcpProductionShadowSeams["startProcess"]>(async () => undefined);
    await expect(startExternalMcpProductionShadow(
      enabledEnv(), shadow(true), temporal(true), foundationAgentTaskActivities,
      {}, undefined, { startProcess }
    )).rejects.toThrow(/^External MCP production Shadow process is unavailable$/);
  });
});

function enabledEnv(): NodeJS.ProcessEnv {
  return {
    DIPOLE_AGENT_EXTERNAL_MCP_ENABLED: "true",
    DIPOLE_AGENT_EXTERNAL_MCP_PROFILES: JSON.stringify([{
      schema_version: "dipole.agent.external-mcp-profile.v1",
      profile_id: "repository-prod", tenant_id: "dipole", server_id: "repository.example",
      endpoint: "https://repository.example/mcp",
      credential: { ref: "CRED-0123456789ABCDEF", version: 1 },
      network_policy: {
        allowed_hosts: ["repository.example"], allowed_ports: [443], dns_resolution: "public_only",
        tls_server_name: "repository.example", ca_bundle_ref: "CA-0123456789ABCDEF"
      },
      allowed_tools: ["get_issue"]
    }])
  };
}

function shadow(enabled: boolean, triggerMode: "direct_target" | "subscription" = "subscription") {
  return loadShadowRuntimeConfig(enabled ? {
    DIPOLE_AGENT_KAFKA_ENABLED: "true", DIPOLE_AGENT_KAFKA_BROKERS: "kafka:9092",
    DIPOLE_AGENT_TRIGGER_MODE: triggerMode,
    DIPOLE_AGENT_CAPABILITY_RPC_ENABLED: "true", DIPOLE_AGENT_CAPABILITY_RPC_TARGET: "127.0.0.1:9091",
    DIPOLE_INTERNAL_RPC_SHARED_SECRET: "test-secret"
  } : {});
}

function temporal(enabled: boolean, modeSelected = true) {
  return loadTemporalRuntimeConfig({
    DIPOLE_AGENT_TEMPORAL_ENABLED: String(enabled),
    DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE: modeSelected ? "external_mcp_shadow" : "foundation"
  });
}
