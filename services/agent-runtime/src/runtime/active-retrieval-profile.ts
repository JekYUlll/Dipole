import type { TemporalRuntimeConfig } from "../temporal/temporal-runtime.js";
import type { ActiveReadProfileSurface } from "./active-read-profile.js";

export interface ActiveRetrievalProfile extends ActiveReadProfileSurface {
  readonly runtimeMode: "shadow" | "active";
  readonly temporal: TemporalRuntimeConfig;
  readonly capabilityRPCEnabled: boolean;
  readonly capabilityRPCTLS: boolean;
  readonly interactiveMessageWritesEnabled: boolean;
  readonly subscriptionMessageWritesEnabled: boolean;
}

// Retrieval runs on a dedicated Worker so its broader read surface cannot be
// accidentally enabled on the interactive or subscription execution paths.
export function assertActiveRetrievalProfile(profile: ActiveRetrievalProfile): void {
  if (profile.runtimeMode !== "active") throw new Error("Retrieval Agent profile requires active Runtime mode");
  if (!profile.temporal.enabled || profile.temporal.activityMode !== "retrieval_active") {
    throw new Error("Retrieval Agent profile requires retrieval_active Temporal Activities");
  }
  if (!profile.temporal.taskQueue.startsWith("dipole-agent-retrieval-")) {
    throw new Error("Retrieval Agent profile requires an isolated retrieval task queue");
  }
  if (!profile.capabilityRPCEnabled || !profile.capabilityRPCTLS) {
    throw new Error("Retrieval Agent profile requires mTLS Agent Capability RPC");
  }
  if (!profile.controlEnabled) throw new Error("Retrieval Agent profile requires Control API");
  if (!profile.retrievalEnabled || !profile.retrievalContextEnabled) {
    throw new Error("Retrieval Agent profile requires retrieval and retrieval Context");
  }
  const forbidden = [
    ["MCP Server", profile.mcpServerEnabled], ["External MCP", profile.externalMcpEnabled],
    ["Memory", profile.memoryEnabled], ["interactive message writes", profile.interactiveMessageWritesEnabled],
    ["subscription message writes", profile.subscriptionMessageWritesEnabled], ["subscription Shadow", profile.subscriptionShadowEnabled]
  ].filter(([, enabled]) => enabled).map(([name]) => name);
  if (forbidden.length > 0) throw new Error(`Retrieval Agent profile forbids: ${forbidden.join(", ")}`);
}
