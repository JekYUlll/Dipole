import type { TemporalRuntimeConfig } from "../temporal/temporal-runtime.js";

import { assertActiveReadProfileSurface, type ActiveReadProfileSurface } from "./active-read-profile.js";

export interface ActiveMemoryPromotionProfile extends ActiveReadProfileSurface {
  readonly runtimeMode: "shadow" | "active";
  readonly temporal: TemporalRuntimeConfig;
  readonly capabilityRPCEnabled: boolean;
  readonly capabilityRPCTLS: boolean;
  readonly commitEnabled: boolean;
  readonly authority: string;
}

export function assertActiveMemoryPromotionProfile(profile: ActiveMemoryPromotionProfile): void {
  if (profile.commitEnabled) {
    if (profile.runtimeMode !== "active") throw new Error("Memory promotion commit requires active Agent Runtime mode");
    if (!profile.temporal.enabled || profile.temporal.activityMode !== "promotion_active") {
      throw new Error("Memory promotion commit requires promotion_active Temporal Activities");
    }
    if (!profile.temporal.taskQueue.startsWith("dipole-agent-memory-promotion-")) {
      throw new Error("Memory promotion commit requires an isolated promotion Temporal task queue");
    }
    if (!profile.capabilityRPCEnabled || !profile.capabilityRPCTLS) {
      throw new Error("Memory promotion commit requires Agent Capability RPC mTLS");
    }
    if (profile.authority !== "operator_approved") {
      throw new Error("Memory promotion commit requires operator-approved authority");
    }
    assertActiveReadProfileSurface(profile.runtimeMode, profile);
    return;
  }
  if (profile.temporal.activityMode === "promotion_active") {
    throw new Error("promotion_active Temporal Activities require the Memory promotion commit switch");
  }
}
