import type { TemporalRuntimeConfig } from "../temporal/temporal-runtime.js";
import type { ActiveReadProfileSurface } from "./active-read-profile.js";

export interface ActiveSubscriptionProfile extends ActiveReadProfileSurface {
  readonly runtimeMode: "shadow" | "active";
  readonly temporal: TemporalRuntimeConfig;
  readonly triggerMode: "direct_target" | "subscription";
  readonly subscriptionActiveEnabled: boolean;
  readonly capabilityRPCEnabled: boolean;
  readonly capabilityRPCTLS: boolean;
  readonly interactiveMessageWritesEnabled: boolean;
}

// Subscription tasks are a separate rollout surface so their consumer group and
// Temporal queue cannot be mistaken for the direct-target read profile.
export function assertActiveSubscriptionProfile(profile: ActiveSubscriptionProfile): void {
  if (profile.runtimeMode !== "active") throw new Error("Subscription Agent profile requires active Runtime mode");
  if (!profile.subscriptionActiveEnabled || profile.triggerMode !== "subscription") {
    throw new Error("Subscription Agent profile requires explicit subscription trigger enablement");
  }
  if (!profile.temporal.enabled || profile.temporal.activityMode !== "subscription_active") {
    throw new Error("Subscription Agent profile requires subscription_active Temporal Activities");
  }
  if (!profile.temporal.taskQueue.startsWith("dipole-agent-subscription-")) {
    throw new Error("Subscription Agent profile requires an isolated subscription task queue");
  }
  if (!profile.capabilityRPCEnabled || !profile.capabilityRPCTLS) {
    throw new Error("Subscription Agent profile requires mTLS Agent Capability RPC");
  }
  if (profile.interactiveMessageWritesEnabled) {
    throw new Error("Subscription Agent profile forbids interactive message writes");
  }
  const forbidden = [
    ["Control API", profile.controlEnabled], ["MCP Server", profile.mcpServerEnabled],
    ["External MCP", profile.externalMcpEnabled], ["Memory", profile.memoryEnabled],
    ["retrieval", profile.retrievalEnabled], ["retrieval Context", profile.retrievalContextEnabled],
    ["subscription Shadow", profile.subscriptionShadowEnabled]
  ].filter(([, enabled]) => enabled).map(([name]) => name);
  if (forbidden.length > 0) throw new Error(`Subscription Agent profile forbids: ${forbidden.join(", ")}`);
}
