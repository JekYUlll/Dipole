import { buildServer } from "./server.js";
import { AgentTaskControlService } from "./control/agent-task-control.js";
import {
  createAgentCapabilityRPC,
  createKafkaShadowRuntime,
  createTemporalReadActivityResources,
  loadShadowRuntimeConfig
} from "./runtime/shadow-runtime.js";
import {
  foundationAgentTaskActivities,
  type AgentTaskWorkerActivities
} from "./temporal/agent-task-activities.js";
import { createPersistentAgentTaskLifecycleActivities } from "./temporal/agent-task-lifecycle-activities.js";
import {
  createTemporalTaskDispatchRuntime,
  type TemporalTaskDispatchRuntime
} from "./temporal/temporal-task-client.js";
import {
  createTemporalWorkerRuntime,
  loadTemporalRuntimeConfig,
  type TemporalWorkerRuntime
} from "./temporal/temporal-runtime.js";

const port = Number.parseInt(process.env.DIPOLE_AGENT_PORT ?? "8091", 10);
const host = process.env.DIPOLE_AGENT_HOST?.trim() || "0.0.0.0";
let ready = false;
const shadowConfig = loadShadowRuntimeConfig(process.env);
const temporalConfig = loadTemporalRuntimeConfig(process.env);
const controlEnabled = process.env.DIPOLE_AGENT_CONTROL_ENABLED?.trim().toLowerCase() === "true";
const controlSecret = process.env.DIPOLE_AGENT_CONTROL_SECRET ?? process.env.DIPOLE_INTERNAL_RPC_SHARED_SECRET ?? "";
if (controlEnabled && (!temporalConfig.enabled || !shadowConfig.capabilityRpc.enabled || controlSecret.trim().length === 0)) {
  throw new Error("Agent Task controls require Temporal, Agent Capability RPC, and a control secret");
}
let temporalRuntime: TemporalWorkerRuntime | undefined;
let temporalRPC: ReturnType<typeof createAgentCapabilityRPC> | undefined;
const controlRPC = controlEnabled ? createAgentCapabilityRPC(shadowConfig) : undefined;
const temporalReadResources = temporalConfig.enabled && temporalConfig.activityMode === "read_shadow"
  ? createTemporalReadActivityResources(shadowConfig)
  : undefined;
let temporalDispatcher: TemporalTaskDispatchRuntime | undefined;
if (temporalConfig.enabled && ((temporalConfig.activityMode === "read_shadow" && shadowConfig.enabled) || controlEnabled)) {
  temporalDispatcher = createTemporalTaskDispatchRuntime(temporalConfig);
}
const shadowRuntime = shadowConfig.enabled ? createKafkaShadowRuntime(shadowConfig, temporalDispatcher) : undefined;
let serverStarted = false;
let shadowStarted = false;
let temporalStarted = false;
let temporalReadResourcesOpen = temporalReadResources !== undefined;
let temporalDispatcherStarted = false;
let stopPromise: Promise<void> | undefined;

const controlService = controlEnabled
  ? new AgentTaskControlService(controlRPC!.client, temporalDispatcher!)
  : undefined;
const server = buildServer(
  { isReady: () => ready },
  controlService === undefined ? undefined : { secret: controlSecret, service: controlService }
);
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
    if (temporalDispatcherStarted && temporalDispatcher !== undefined) {
      try {
        await temporalDispatcher.stop();
      } catch (error) {
        failures.push(error);
      }
      temporalDispatcherStarted = false;
    }
    if (temporalStarted && temporalRuntime !== undefined) {
      try {
        await temporalRuntime.stop();
      } catch (error) {
        failures.push(error);
      }
      temporalStarted = false;
    }
    if (temporalReadResourcesOpen && temporalReadResources !== undefined) {
      try {
        await temporalReadResources.stop();
      } catch (error) {
        failures.push(error);
      }
      temporalReadResourcesOpen = false;
    }
    temporalRPC?.close();
    temporalRPC = undefined;
    controlRPC?.close();
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
  } else if (temporalConfig.activityMode === "read_shadow") {
    activities = {
      ...foundationAgentTaskActivities,
      ...createPersistentAgentTaskLifecycleActivities(temporalReadResources!.client),
      ...temporalReadResources!.activities
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
  if (temporalReadResources !== undefined) {
    await temporalReadResources.start();
  }
  if (temporalRuntime !== undefined) {
    temporalStarted = true;
    await temporalRuntime.start();
  }
  if (temporalDispatcher !== undefined) {
    temporalDispatcherStarted = true;
    await temporalDispatcher.start();
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
