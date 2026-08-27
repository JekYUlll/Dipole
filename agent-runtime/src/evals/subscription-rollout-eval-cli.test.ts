import { describe, expect, it } from "vitest";

import { runSubscriptionRolloutEvalCLI } from "./subscription-rollout-eval-cli.js";

describe("subscription rollout eval CLI", () => {
  it("recomputes the synthetic source evidence and emits a low-sensitive decision", async () => {
    const root = new URL("../../../contracts/agent-subscription-prefilter/v1/", import.meta.url);
    const output: string[] = [];
    const errors: string[] = [];
    const code = await runSubscriptionRolloutEvalCLI([
      `--corpus=${new URL("corpus.example.json", root).pathname}`,
      `--review=${new URL("review.example.json", root).pathname}`,
      `--evidence=${new URL("evidence.example.json", root).pathname}`
    ], { write: value => output.push(value) }, { write: value => errors.push(value) });
    expect(code).toBe(0);
    expect(errors).toEqual([]);
    expect(JSON.parse(output.join(""))).toMatchObject({ decision: "eligible", reasons: [], candidate: { kind: "rule" } });
    expect(output.join("")).not.toContain("Incident detected");
    expect(output.join("")).not.toContain("reviewer:alice");
  });

  it("uses exit 1 for invalid arguments", async () => {
    const errors: string[] = [];
    expect(await runSubscriptionRolloutEvalCLI([], { write: () => undefined }, { write: value => errors.push(value) })).toBe(1);
    expect(errors.join("")).toMatch(/requires/iu);
  });
});
