import { readFile } from "node:fs/promises";

import { verifyAgentWorkflowRepairPreflight } from "./agent-workflow-repair-execution-plan.js";

const inputPath = requiredArgument("--input");
const input = JSON.parse(await readFile(inputPath, "utf8"));
const receipt = verifyAgentWorkflowRepairPreflight(input);
process.stdout.write(`${JSON.stringify(receipt, null, 2)}\n`);
if (receipt.decision === "blocked") process.exitCode = 2;

function requiredArgument(name: string): string {
  const prefix = `${name}=`;
  const value = process.argv.slice(2).find((argument) => argument.startsWith(prefix))?.slice(prefix.length).trim();
  if (!value) throw new Error(`${name} is required`);
  return value;
}
