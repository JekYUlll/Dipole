import { createHash } from "node:crypto";

import { canonicalJSON } from "./offline-evaluator.js";
import type { MemoryPrefilterRolloutDecision } from "./memory-prefilter-rollout.js";

export type MemoryPrefilterRuntimeMode = "off" | "shadow" | "enforced";

export interface MemoryPrefilterRuntimeBinding {
  readonly mode: MemoryPrefilterRuntimeMode;
  readonly decision: MemoryPrefilterRolloutDecision;
  readonly decisionSha256: string;
  readonly candidateId: string;
  readonly configurationSha256: string;
  readonly corpusSha256: string;
  readonly reviewSha256: string;
}

export interface MemoryPrefilterRuntimeResult {
  readonly mode: MemoryPrefilterRuntimeMode;
  readonly outcome: "bypassed" | "observed" | "admitted" | "blocked";
  readonly taskCreationAllowed: boolean;
  readonly memoryWriteAuthority: false;
  readonly reason: string;
}

export class MemoryPrefilterRuntimeGate {
  constructor(private readonly binding: MemoryPrefilterRuntimeBinding) {}

  evaluate(): MemoryPrefilterRuntimeResult {
    if (this.binding.mode === "off") return this.result("bypassed", true, "prefilter_disabled");
    this.verifyBinding();
    if (this.binding.mode === "shadow") return this.result("observed", true, "prefilter_shadow_only");
    if (this.binding.decision.decision !== "eligible") return this.result("blocked", false, "prefilter_rollout_blocked");
    return this.result("admitted", true, "prefilter_rollout_eligible");
  }

  private verifyBinding(): void {
    const decision = this.binding.decision;
    if (digest(decision) !== this.binding.decisionSha256) throw new Error("memory prefilter rollout decision hash drift");
    if (decision.candidate.id !== this.binding.candidateId || decision.candidate.configurationSha256 !== this.binding.configurationSha256) {
      throw new Error("memory prefilter candidate binding drift");
    }
    if (decision.corpusSha256 !== this.binding.corpusSha256 || decision.reviewSha256 !== this.binding.reviewSha256) {
      throw new Error("memory prefilter corpus binding drift");
    }
  }

  private result(outcome: MemoryPrefilterRuntimeResult["outcome"], taskCreationAllowed: boolean, reason: string): MemoryPrefilterRuntimeResult {
    return { mode: this.binding.mode, outcome, taskCreationAllowed, memoryWriteAuthority: false, reason };
  }
}

function digest(value: unknown): string { return createHash("sha256").update(canonicalJSON(value)).digest("hex"); }
