import { describe, expect, it } from "vitest";

import { runContextAblationBindingCLI } from "./context-ablation-binding-cli.js";

describe("Context ablation binding CLI", () => {
  it("requires an owner-reviewed source before opening the binding account", async () => {
    const errors: string[] = [];
    const code = await runContextAblationBindingCLI(
      ["--manifest=/secure/manifest.json", "--bindings=/secure/bindings.json"],
      { write: () => true },
      { write: value => { errors.push(String(value)); return true; } }
    );
    expect(code).toBe(1);
    expect(errors.join("")).toContain("--reviewed-source");
  });
});
