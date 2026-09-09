import { describe, expect, it } from "vitest";

import { vi } from "vitest";

import {
  createTemporalWorkerRuntime,
  loadTemporalRuntimeConfig,
  type TemporalWorkerState
} from "./temporal-runtime.js";
import { foundationAgentTaskActivities } from "./agent-task-activities.js";

describe("Temporal runtime configuration", () => {
  it("keeps the worker disabled by default", () => {
    expect(loadTemporalRuntimeConfig({})).toEqual({
      enabled: false,
      address: "127.0.0.1:7233",
      namespace: "default",
      taskQueue: "dipole-agent-task-v1",
      runtimeMode: "shadow"
    });
  });

  it("loads an explicit no-cutover worker profile", () => {
    expect(loadTemporalRuntimeConfig({
      DIPOLE_AGENT_TEMPORAL_ENABLED: "true",
      DIPOLE_AGENT_TEMPORAL_ADDRESS: "temporal:7233",
      DIPOLE_AGENT_TEMPORAL_NAMESPACE: "dipole",
      DIPOLE_AGENT_TEMPORAL_TASK_QUEUE: "dipole-agent-task-canary-v1",
      DIPOLE_AGENT_RUNTIME_MODE: "shadow"
    })).toEqual({
      enabled: true,
      address: "temporal:7233",
      namespace: "dipole",
      taskQueue: "dipole-agent-task-canary-v1",
      runtimeMode: "shadow"
    });
  });

  it("loads the default-off Temporal read shadow profile", () => {
    expect(loadTemporalRuntimeConfig({
      DIPOLE_AGENT_TEMPORAL_ENABLED: "true",
      DIPOLE_AGENT_RUNTIME_MODE: "shadow"
    })).toMatchObject({ enabled: true, runtimeMode: "shadow" });
  });

  it("loads the explicit active read Activity profile", () => {
    expect(loadTemporalRuntimeConfig({
      DIPOLE_AGENT_TEMPORAL_ENABLED: "true",
      DIPOLE_AGENT_RUNTIME_MODE: "active"
    })).toMatchObject({ enabled: true, runtimeMode: "active" });
  });

  it("maps the legacy read_active activity setting to active mode", () => {
    expect(loadTemporalRuntimeConfig({
      DIPOLE_AGENT_TEMPORAL_ENABLED: "true",
      DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE: "read_active"
    })).toMatchObject({ enabled: true, runtimeMode: "active" });
  });

  it("maps legacy interactive profiles to the unified active mode", () => {
    expect(loadTemporalRuntimeConfig({
      DIPOLE_AGENT_TEMPORAL_ENABLED: "true",
      DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE: "interactive_active"
    })).toMatchObject({ enabled: true, runtimeMode: "active" });
  });

  it("rejects empty required values when enabled", () => {
    expect(() => loadTemporalRuntimeConfig({
      DIPOLE_AGENT_TEMPORAL_ENABLED: "true",
      DIPOLE_AGENT_TEMPORAL_ADDRESS: " "
    })).toThrow(/address/);
  });

  it("starts polling before readiness and closes the connection after shutdown", async () => {
    let state: TemporalWorkerState = "INITIALIZED";
    let finishRun: (() => void) | undefined;
    const run = vi.fn(async () => {
      state = "RUNNING";
      await new Promise<void>((resolve) => { finishRun = resolve; });
      state = "STOPPED";
    });
    const shutdown = vi.fn(() => { finishRun?.(); });
    const close = vi.fn(async () => undefined);
    const create = vi.fn(async () => ({
      worker: { run, shutdown, getState: () => state }, close
    }));
    const config = loadTemporalRuntimeConfig({ DIPOLE_AGENT_TEMPORAL_ENABLED: "true" });
    const runtime = createTemporalWorkerRuntime(config, foundationAgentTaskActivities, { create });

    await runtime.start();
    expect(create).toHaveBeenCalledWith(config, foundationAgentTaskActivities);
    expect(run).toHaveBeenCalledOnce();
    await runtime.stop();
    expect(shutdown).toHaveBeenCalledOnce();
    expect(close).toHaveBeenCalledOnce();
  });
});
