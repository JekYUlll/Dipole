import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import {
  evaluateAgentShadowPromotion, evaluateAgentShadowPromotionV2,
  parseAgentShadowPromotionEvidence, parseAgentShadowPromotionEvidenceV2
} from "./agent-shadow-promotion-policy.js";

interface Writable {
  write(value: string): unknown;
}

export async function runPromotionCheckCLI(args: string[], stdout: Writable, stderr: Writable): Promise<number> {
  const evidenceArgs = args.filter(argument => argument.startsWith("--evidence="));
  if (args.length !== 1 || evidenceArgs.length !== 1 || evidenceArgs[0]!.slice("--evidence=".length).trim() === "") {
    stderr.write("promotion check requires exactly one --evidence=<path> argument\n");
    return 1;
  }

  try {
    const value = JSON.parse(await readFile(evidenceArgs[0]!.slice("--evidence=".length), "utf8")) as { schemaVersion?: unknown };
    const decision = value.schemaVersion === "dipole.agent.shadow-promotion-evidence.v2"
      ? evaluateAgentShadowPromotionV2(parseAgentShadowPromotionEvidenceV2(value))
      : evaluateAgentShadowPromotion(parseAgentShadowPromotionEvidence(value));
    stdout.write(`${JSON.stringify(decision, null, 2)}\n`);
    return decision.decision === "eligible" ? 0 : 2;
  } catch (error) {
    stderr.write(`promotion evidence is invalid: ${error instanceof Error ? error.message : String(error)}\n`);
    return 1;
  }
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runPromotionCheckCLI(process.argv.slice(2), process.stdout, process.stderr);
}
