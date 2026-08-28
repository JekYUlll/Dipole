import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { runSubscriptionShadowEvidenceCLI } from "./subscription-shadow-evidence-cli.js";

describe("subscription Shadow evidence CLI", () => {
  it("creates and independently verifies a low-sensitive receipt", async () => {
    const dir = await mkdtemp(join(tmpdir(), "dipole=sub-shadow-"));
    try {
      const inputPath = join(dir, "input.json"), evidencePath = join(dir, "evidence.json");
      await writeFile(inputPath, JSON.stringify(input()), { mode: 0o600 });
      const created = sink(), error = sink(), now = () => new Date("2026-08-29T01:00:00.000Z");
      expect(await runSubscriptionShadowEvidenceCLI([`--input=${inputPath}`], created, error, now)).toBe(0);
      await writeFile(evidencePath, created.value, { mode: 0o600 });
      const receipt = sink();
      expect(await runSubscriptionShadowEvidenceCLI([`--evidence=${evidencePath}`], receipt, error, () => new Date("2026-08-29T02:00:00.000Z"))).toBe(0);
      expect(JSON.parse(receipt.value)).toMatchObject({ outcome: "passed", observed_events: 100, production_authority: false, runtime_change_authority: false });
      expect(receipt.value).not.toContain(inputPath);
    } finally { await rm(dir, { recursive: true, force: true }); }
  });

  it("uses fixed errors for invalid arguments and tampered files", async () => {
    const output = sink(), error = sink();
    expect(await runSubscriptionShadowEvidenceCLI([], output, error)).toBe(1);
    expect(await runSubscriptionShadowEvidenceCLI(["--evidence=/missing"], output, error)).toBe(1);
    expect(error.value).not.toContain("/missing");
  });
});

function sink() { return { value: "", write(value: string) { this.value += value; } }; }
function input() {
  return {
    window_start: "2026-08-28T00:00:00.000Z", window_end: "2026-08-29T00:00:00.000Z",
    runtime_revision: "a".repeat(64), prometheus_config_sha256: "b".repeat(64), query_revision: "subscription-shadow-v1",
    expected_scrapes: 17_280, successful_scrapes: 17_000, counter_resets: 0,
    start: { accepted_match: 0, accepted_miss: 0, accepted_error: 0, ignored_match: 0, ignored_miss: 0, ignored_error: 0, candidates: 0 },
    end: { accepted_match: 25, accepted_miss: 25, accepted_error: 0, ignored_match: 25, ignored_miss: 25, ignored_error: 0, candidates: 50 }
  };
}
