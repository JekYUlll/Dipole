import { buildServer } from "./server.js";
import {
  createAgentCapabilityRPC,
  createKafkaShadowRuntime,
  loadShadowRuntimeConfig
} from "./runtime/shadow-runtime.js";
import {
  foundationAgentTaskActivities,
  type AgentTaskWorkerActivities
} from "./temporal/agent-task-activities.js";
import { createPersistentAgentTaskLifecycleActivities } from "./temporal/agent-task-lifecycle-activities.js";
import {
  createTemporalWorkerRuntime,
  loadTemporalRuntimeConfig,
  type TemporalWorkerRuntime
} from "./temporal/temporal-runtime.js";

const port = Number.parseInt(process.env.DIPOLE_AGENT_PORT ?? "8091", 10);
const host = process.env.DIPOLE_AGENT_HOST?.trim() || "0.0.0.0";
let ready = false;
const shadowConfig = loadShadowRuntimeConfig(process.env);
const shadowRuntime = shadowConfig.enabled ? createKafkaShadowRuntime(shadowConfig) : undefined;
const temporalConfig = loadTemporalRuntimeConfig(process.env);
let temporalRuntime: TemporalWorkerRuntime | undefined;
let temporalRPC: ReturnType<typeof createAgentCapabilityRPC> | undefined;
let serverStarted = false;
let shadowStarted = false;
let temporalStarted = false;
let stopPromise: Promise<void> | undefined;

const server = buildServer({ isReady: () => ready });
const stop = (): Promise<void> => {
  stopPromise ??= (async () => {
    ready = false;
    const failures: unknown[] = [];
    if (shadowStarted && shadowRuntime !== undefined) {
      try {
        await shadowRuntime.stop();
      } catch (error) {
        failures.push(error);
      }
      shadowStarted = false;
    }
    if (temporalStarted && temporalRuntime !== undefined) {
      try {
        await temporalRuntime.stop();
      } catch (error) {
        failures.push(error);
      }
      temporalStarted = false;
    }
    temporalRPC?.close();
    temporalRPC = undefined;
    if (serverStarted) {
      try {
        await server.close();
      } catch (error) {
        failures.push(error);
      }
      serverStarted = false;
    }
    if (failures.length > 0) {
      throw new AggregateError(failures, "Agent Runtime shutdown failed");
    }
  })();
  return stopPromise;
};

if (temporalConfig.enabled) {
  let activities: AgentTaskWorkerActivities = foundationAgentTaskActivities;
  if (temporalConfig.activityMode === "persistent_shadow") {
    if (!shadowConfig.capabilityRpc.enabled) {
      throw new Error("Persistent Temporal shadow Activities require Agent Capability RPC");
    }
    temporalRPC = createAgentCapabilityRPC(shadowConfig);
    activities = {
      ...foundationAgentTaskActivities,
      ...createPersistentAgentTaskLifecycleActivities(temporalRPC.client)
    };
  }
  temporalRuntime = createTemporalWorkerRuntime(
    temporalConfig,
    activities,
    undefined,
    (error) => {
      if (!ready) {
        return;
      }
      process.stderr.write(`Temporal Worker failed: ${String(error)}\n`);
      process.exitCode = 1;
      void stop().catch((stopError: unknown) => {
        process.stderr.write(`${String(stopError)}\n`);
      });
    }
  );
}

try {
  await server.listen({ host, port });
  serverStarted = true;
  if (temporalRuntime !== undefined) {
    temporalStarted = true;
    await temporalRuntime.start();
  }
  if (shadowRuntime !== undefined) {
    shadowStarted = true;
    await shadowRuntime.start();
  }
  ready = true;
} catch (error) {
  process.stderr.write(`${String(error)}\n`);
  try {
    await stop();
  } catch (stopError) {
    process.stderr.write(`${String(stopError)}\n`);
  }
  process.exitCode = 1;
}

for (const signal of ["SIGINT", "SIGTERM"] as const) {
  process.once(signal, () => {
    void stop().catch((error: unknown) => {
      process.stderr.write(`${String(error)}\n`);
      process.exitCode = 1;
    });
  });
}
