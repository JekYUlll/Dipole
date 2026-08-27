import { readFile } from "node:fs/promises";

import { evaluateAgentShadowPromotion, parseAgentShadowPromotionEvidence } from "./agent-shadow-promotion-policy.js";

const evidencePath = requiredArgument("--evidence");
const evidence = parseAgentShadowPromotionEvidence(JSON.parse(await readFile(evidencePath, "utf8")));
const decision = evaluateAgentShadowPromotion(evidence);
process.stdout.write(`${JSON.stringify(decision, null, 2)}\n`);
if (decision.decision !== "eligible") process.exitCode = 2;

function requiredArgument(name: string): string {
  const prefix = `${name}=`;
  const value = process.argv.slice(2).find((argument) => argument.startsWith(prefix))?.slice(prefix.length).trim();
  if (!value) throw new Error(`${name} is required`);
  return value;
}
