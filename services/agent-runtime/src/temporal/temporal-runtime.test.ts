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
      activityMode: "foundation"
    });
  });

  it("loads an explicit no-cutover worker profile", () => {
    expect(loadTemporalRuntimeConfig({
      DIPOLE_AGENT_TEMPORAL_ENABLED: "true",
      DIPOLE_AGENT_TEMPORAL_ADDRESS: "temporal:7233",
      DIPOLE_AGENT_TEMPORAL_NAMESPACE: "dipole",
      DIPOLE_AGENT_TEMPORAL_TASK_QUEUE: "dipole-agent-task-canary-v1",
      DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE: "persistent_shadow"
    })).toEqual({
      enabled: true,
      address: "temporal:7233",
      namespace: "dipole",
      taskQueue: "dipole-agent-task-canary-v1",
      activityMode: "persistent_shadow"
    });
  });

  it("loads the default-off Temporal read shadow profile", () => {
    expect(loadTemporalRuntimeConfig({
      DIPOLE_AGENT_TEMPORAL_ENABLED: "true",
      DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE: "read_shadow"
    })).toMatchObject({ enabled: true, activityMode: "read_shadow" });
  });

  it("loads the explicit active read Activity profile", () => {
    expect(loadTemporalRuntimeConfig({
      DIPOLE_AGENT_TEMPORAL_ENABLED: "true",
      DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE: "read_active"
    })).toMatchObject({ enabled: true, activityMode: "read_active" });
  });

  it("loads the explicit active Memory promotion Activity profile", () => {
    expect(loadTemporalRuntimeConfig({
      DIPOLE_AGENT_TEMPORAL_ENABLED: "true",
      DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE: "promotion_active"
    })).toMatchObject({ enabled: true, activityMode: "promotion_active" });
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
