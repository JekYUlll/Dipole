import { z } from "zod";
import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";

import type { AgentTaskWorkerActivities } from "./agent-task-activities.js";
import type { TemporalMcpDispatchActivities } from "./mcp-dispatch-activity.js";

export type TemporalWorkerActivities = AgentTaskWorkerActivities & Partial<TemporalMcpDispatchActivities>;

const temporalRuntimeConfigSchema = z.object({
  enabled: z.boolean(),
  address: z.string().trim().min(1),
  namespace: z.string().trim().min(1),
  taskQueue: z.string().trim().min(1),
  activityMode: z.enum(["foundation", "persistent_shadow", "read_shadow", "read_active", "external_mcp_shadow"])
}).strict();

export type TemporalRuntimeConfig = z.infer<typeof temporalRuntimeConfigSchema>;

export type TemporalWorkerState = "INITIALIZED" | "RUNNING" | "STOPPED" | "STOPPING" | "DRAINING" | "DRAINED" | "FAILED";

interface TemporalWorkerPort {
  run(): Promise<void>;
  shutdown(): void;
  getState(): TemporalWorkerState;
}

export interface TemporalWorkerFactoryPort {
  create(config: TemporalRuntimeConfig, activities: TemporalWorkerActivities): Promise<{
    worker: TemporalWorkerPort;
    close(): Promise<void>;
  }>;
}

export interface TemporalWorkerRuntime {
  start(): Promise<void>;
  stop(): Promise<void>;
}

export function loadTemporalRuntimeConfig(env: NodeJS.ProcessEnv): TemporalRuntimeConfig {
  return temporalRuntimeConfigSchema.parse({
    enabled: env.DIPOLE_AGENT_TEMPORAL_ENABLED?.trim().toLowerCase() === "true",
    address: env.DIPOLE_AGENT_TEMPORAL_ADDRESS === undefined ? "127.0.0.1:7233" : env.DIPOLE_AGENT_TEMPORAL_ADDRESS,
    namespace: env.DIPOLE_AGENT_TEMPORAL_NAMESPACE === undefined ? "default" : env.DIPOLE_AGENT_TEMPORAL_NAMESPACE,
    taskQueue: env.DIPOLE_AGENT_TEMPORAL_TASK_QUEUE === undefined
      ? "dipole-agent-task-v1"
      : env.DIPOLE_AGENT_TEMPORAL_TASK_QUEUE,
    activityMode: env.DIPOLE_AGENT_TEMPORAL_ACTIVITY_MODE?.trim().toLowerCase() || "foundation"
  });
}

export function createTemporalWorkerRuntime(
  config: TemporalRuntimeConfig,
  activities: TemporalWorkerActivities,
  factory: TemporalWorkerFactoryPort = nativeTemporalWorkerFactory,
  onFailure: (error: unknown) => void = () => undefined
): TemporalWorkerRuntime {
  let resources: Awaited<ReturnType<TemporalWorkerFactoryPort["create"]>> | undefined;
  let runPromise: Promise<void> | undefined;
  let runFailure: unknown;

  return {
    start: async () => {
      if (resources !== undefined) {
        throw new Error("Temporal Worker Runtime is already started");
      }
      resources = await factory.create(config, activities);
      runPromise = resources.worker.run().catch((error: unknown) => {
        runFailure = error;
        onFailure(error);
      });
      try {
        await waitUntilRunning(resources.worker, () => runFailure);
      } catch (error) {
        await stopResources(resources, runPromise);
        resources = undefined;
        runPromise = undefined;
        throw error;
      }
    },
    stop: async () => {
      if (resources === undefined) {
        return;
      }
      await stopResources(resources, runPromise);
      resources = undefined;
      runPromise = undefined;
    }
  };
}

const nativeTemporalWorkerFactory: TemporalWorkerFactoryPort = {
  async create(config, activities) {
    const { NativeConnection, Worker } = await import("@temporalio/worker");
    const connection = await NativeConnection.connect({ address: config.address });
    try {
      const worker = await Worker.create({
        connection,
        namespace: config.namespace,
        taskQueue: config.taskQueue,
        workflowsPath: workflowModulePath(),
        activities: { ...activities }
      });
      return { worker, close: () => connection.close() };
    } catch (error) {
      await connection.close();
      throw error;
    }
  }
};

function workflowModulePath(): string {
  const compiled = fileURLToPath(new URL("./agent-task-workflow.js", import.meta.url));
  if (existsSync(compiled)) {
    return compiled;
  }
  return fileURLToPath(new URL("./agent-task-workflow.ts", import.meta.url));
}

async function waitUntilRunning(worker: TemporalWorkerPort, failure: () => unknown): Promise<void> {
  for (let attempt = 0; attempt < 500; attempt += 1) {
    if (worker.getState() === "RUNNING") {
      return;
    }
    const cause = failure();
    if (cause !== undefined) {
      throw cause;
    }
    if (worker.getState() === "FAILED" || worker.getState() === "STOPPED") {
      throw new Error(`Temporal Worker stopped during startup with state ${worker.getState()}`);
    }
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  throw new Error("Temporal Worker did not enter RUNNING within 5 seconds");
}

async function stopResources(
  resources: Awaited<ReturnType<TemporalWorkerFactoryPort["create"]>>,
  runPromise: Promise<void> | undefined
): Promise<void> {
  const state = resources.worker.getState();
  try {
    if (state !== "STOPPED" && state !== "STOPPING" && state !== "DRAINING" && state !== "DRAINED" && state !== "FAILED") {
      resources.worker.shutdown();
    }
    await runPromise;
  } finally {
    await resources.close();
  }
}
