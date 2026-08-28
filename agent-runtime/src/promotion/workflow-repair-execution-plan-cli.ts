import { readFile } from "node:fs/promises";

import { createAgentWorkflowRepairExecutionPlan } from "./agent-workflow-repair-execution-plan.js";

const inputPath = requiredArgument("--input");
const input = JSON.parse(await readFile(inputPath, "utf8"));
process.stdout.write(`${JSON.stringify(createAgentWorkflowRepairExecutionPlan(input), null, 2)}\n`);

function requiredArgument(name: string): string {
  const prefix = `${name}=`;
  const value = process.argv.slice(2).find((argument) => argument.startsWith(prefix))?.slice(prefix.length).trim();
  if (!value) throw new Error(`${name} is required`);
  return value;
}
