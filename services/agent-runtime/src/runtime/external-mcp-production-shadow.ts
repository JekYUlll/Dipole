import { loadExternalMcpConfig } from "../mcp/external-mcp-profile.js";
import type { ExternalMcpDeploymentPlanOptions } from "../mcp/external-mcp-deployment-composition.js";
import type { AgentTaskWorkerActivities } from "../temporal/agent-task-activities.js";
import { createExternalMcpSubscriptionRouteSelector } from "../temporal/mcp-subscription-route-selector.js";
import type { TemporalRuntimeConfig } from "../temporal/temporal-runtime.js";
import {
  startExternalMcpShadowProcess,
  type ExternalMcpShadowProcess
} from "./external-mcp-shadow-process.js";
import type { ShadowRuntimeConfig } from "./shadow-runtime.js";

export interface ExternalMcpProductionShadowSeams {
  readonly startProcess: typeof startExternalMcpShadowProcess;
}

const defaultSeams: ExternalMcpProductionShadowSeams = {
  startProcess: startExternalMcpShadowProcess
};

export function validateExternalMcpProductionShadowMode(
  env: NodeJS.ProcessEnv,
  shadow: ShadowRuntimeConfig,
  temporal: TemporalRuntimeConfig
): boolean {
  const externalEnabled = loadExternalMcpConfig(env).enabled;
  const modeSelected = temporal.activityMode === "external_mcp_shadow";
  if (!externalEnabled && !modeSelected) return false;
  if (!externalEnabled || !modeSelected || !temporal.enabled || !shadow.enabled ||
      shadow.triggerMode !== "subscription" || !shadow.capabilityRpc.enabled) {
    throw new Error("External MCP production Shadow mode configuration is invalid");
  }
  return true;
}

export async function startExternalMcpProductionShadow(
  env: NodeJS.ProcessEnv,
  shadow: ShadowRuntimeConfig,
  temporal: TemporalRuntimeConfig,
  baseActivities: AgentTaskWorkerActivities,
  options: ExternalMcpDeploymentPlanOptions = {},
  onFailure: (error: unknown) => void = () => undefined,
  seams: ExternalMcpProductionShadowSeams = defaultSeams
): Promise<ExternalMcpShadowProcess | undefined> {
  if (!validateExternalMcpProductionShadowMode(env, shadow, temporal)) return undefined;
  const process = await seams.startProcess(
    env,
    shadow,
    temporal,
    baseActivities,
    createExternalMcpSubscriptionRouteSelector,
    options,
    onFailure
  );
  if (process === undefined) {
    throw new Error("External MCP production Shadow process is unavailable");
  }
  return process;
}
