import type { TemporalRuntimeConfig } from "../temporal/temporal-runtime.js";
import type { ActiveReadProfileSurface } from "./active-read-profile.js";

export interface ActiveInteractiveProfile extends ActiveReadProfileSurface {
  readonly runtimeMode: "shadow" | "active";
  readonly temporal: TemporalRuntimeConfig;
  readonly capabilityRPCEnabled: boolean;
  readonly capabilityRPCTLS: boolean;
  readonly interactiveMessageWritesEnabled: boolean;
}

export function assertActiveInteractiveProfile(profile: ActiveInteractiveProfile): void {
  if (profile.runtimeMode !== "active") throw new Error("Interactive Agent profile requires active Runtime mode");
  if (!profile.temporal.enabled || profile.temporal.activityMode !== "interactive_active") {
    throw new Error("Interactive Agent profile requires interactive_active Temporal Activities");
  }
  if (!profile.temporal.taskQueue.startsWith("dipole-agent-interactive-")) {
    throw new Error("Interactive Agent profile requires an isolated interactive task queue");
  }
  if (!profile.capabilityRPCEnabled || !profile.capabilityRPCTLS) {
    throw new Error("Interactive Agent profile requires mTLS Agent Capability RPC");
  }
  if (!profile.controlEnabled || !profile.interactiveMessageWritesEnabled) {
    throw new Error("Interactive Agent profile requires Control API and explicit message write enablement");
  }
  const forbidden = [
    ["MCP Server", profile.mcpServerEnabled], ["External MCP", profile.externalMcpEnabled],
    ["Memory", profile.memoryEnabled], ["retrieval", profile.retrievalEnabled],
    ["retrieval Context", profile.retrievalContextEnabled], ["subscription Shadow", profile.subscriptionShadowEnabled]
  ].filter(([, enabled]) => enabled).map(([name]) => name);
  if (forbidden.length > 0) throw new Error(`Interactive Agent profile forbids: ${forbidden.join(", ")}`);
}
