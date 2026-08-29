import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it, vi } from "vitest";

import { runSubscriptionShadowCollectorCLI } from "./subscription-shadow-collector-cli.js";

describe("subscription Shadow collector CLI", () => {
  it("emits only the collected evidence input", async () => {
    const directory = await mkdtemp(join(tmpdir(), "dipole=shadow-collector-"));
    try {
      const path = join(directory, "request.json");
      await writeFile(path, JSON.stringify({ request: true }), { mode: 0o600 });
      const collect = vi.fn(async () => ({ window_start: "2026-08-28T00:00:00.000Z", query_revision: "subscription-shadow-v1" }));
      const output = sink(), error = sink();
      expect(await runSubscriptionShadowCollectorCLI([`--request=${path}`], output, error, collect)).toBe(0);
      expect(JSON.parse(output.value)).toEqual({ window_start: "2026-08-28T00:00:00.000Z", query_revision: "subscription-shadow-v1" });
      expect(collect).toHaveBeenCalledWith({ request: true });
      expect(output.value).not.toContain("prometheus_url");
    } finally { await rm(directory, { recursive: true, force: true }); }
  });

  it("uses fixed low-sensitive errors", async () => {
    const output = sink(), error = sink();
    expect(await runSubscriptionShadowCollectorCLI([], output, error)).toBe(1);
    expect(await runSubscriptionShadowCollectorCLI(["--request=/secret/missing"], output, error)).toBe(1);
    expect(error.value).not.toContain("/secret/missing");
  });
});

function sink() { return { value: "", write(value: string) { this.value += value; } }; }
