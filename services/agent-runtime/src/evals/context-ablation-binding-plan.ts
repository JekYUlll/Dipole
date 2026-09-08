import { z } from "zod";

import { parseContextAblationManifest, type ContextAblationManifest } from "./context-ablation-adapter.js";

const id = z.string().trim().min(2).max(128).regex(/^[A-Za-z0-9][A-Za-z0-9._:-]*$/u);
const sha256 = z.string().regex(/^[a-f0-9]{64}$/u);
const condition = z.enum(["baseline", "retrieval", "memory"]);
const binding = z.object({ caseSha256: sha256, condition, taskId: id, runId: id }).strict();
const input = z.object({
  schemaVersion: z.literal("dipole.agent.context-ablation-binding-plan.v1"),
  experimentId: id,
  candidateVersion: z.string().trim().min(2).max(128),
  bindings: z.array(binding).min(3).max(768)
}).strict();

export type ContextAblationBindingPlan = z.infer<typeof input>;

/** Validates the exact case/condition matrix before any operator write occurs. */
export function buildContextAblationBindingPlan(
  rawManifest: ContextAblationManifest | unknown,
  rawPlan: unknown
): ContextAblationBindingPlan {
  const manifest = parseContextAblationManifest(rawManifest);
  const plan = input.parse(typeof rawPlan === "string" ? JSON.parse(rawPlan) as unknown : rawPlan);
  if (plan.experimentId !== manifest.experimentId) throw new Error("Context ablation binding experiment ID does not match the manifest");
  if (plan.candidateVersion !== manifest.candidateVersion) throw new Error("Context ablation binding candidate version does not match the manifest");
  const expected = new Set(manifest.cases.flatMap(item => condition.options.map(name => `${item.caseSha256}\u0000${name}`)));
  const actual = new Set(plan.bindings.map(item => `${item.caseSha256}\u0000${item.condition}`));
  if (actual.size !== plan.bindings.length) throw new Error("Context ablation binding plan contains duplicate case conditions");
  if (actual.size !== expected.size || [...actual].some(key => !expected.has(key))) {
    throw new Error("Context ablation binding plan must cover exactly every manifest case condition");
  }
  if (new Set(plan.bindings.map(item => item.runId)).size !== plan.bindings.length) throw new Error("Context ablation binding plan run IDs must be unique");
  return { ...plan, bindings: [...plan.bindings].sort((left, right) => `${left.caseSha256}\u0000${left.condition}`.localeCompare(`${right.caseSha256}\u0000${right.condition}`)) };
}
