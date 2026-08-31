import { readFile } from "node:fs/promises";

import { describe, expect, it } from "vitest";

import { parseContextAblationManifest } from "./context-ablation-adapter.js";

const contracts = new URL("../../../../contracts/agent-evals/v1/", import.meta.url);
const grants = new URL("../../../../configs/mysql/agent-eval-grants.dist.sql", import.meta.url);

describe("Context Ablation contract", () => {
  it("keeps the public example parseable and low-sensitive", async () => {
    const source = await readFile(new URL("context-ablation-manifest.example.json", contracts), "utf8");
    const manifest = parseContextAblationManifest(source);
    const schema = JSON.parse(await readFile(new URL("context-ablation-manifest.schema.json", contracts), "utf8"));

    expect(manifest).toMatchObject({ schemaVersion: "dipole.agent.context-ablation-manifest.v1", experimentId: "example:context-ablation", cases: [{ requiredOutputIds: ["artifact:conversation_digest:v1"] }] });
    expect(schema.$id).toBe("https://dipole.local/contracts/agent-evals/v1/context-ablation-manifest.schema.json");
    expect(source).not.toMatch(/conversation body|prompt|model output/i);
  });

  it("grants the evaluation account binding reads without a write privilege", async () => {
    const source = await readFile(grants, "utf8");
    expect(source).toContain("GRANT SELECT ON dipole.agent_context_ablation_bindings TO 'dipole_agent_eval'@'%';");
    expect(source).not.toMatch(/GRANT[^;]*(INSERT|UPDATE|DELETE)[^;]*agent_context_ablation_bindings/i);
  });
});
