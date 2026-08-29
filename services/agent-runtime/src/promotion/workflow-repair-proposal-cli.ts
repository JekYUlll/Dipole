import { readFile } from "node:fs/promises";

import { createAgentWorkflowRepairProposal, parseAgentWorkflowRepairProposalInput } from "./agent-workflow-repair-proposal.js";

const inputPath = requiredArgument("--input");
const input = parseAgentWorkflowRepairProposalInput(JSON.parse(await readFile(inputPath, "utf8")));
const proposal = createAgentWorkflowRepairProposal(input);
process.stdout.write(`${JSON.stringify(proposal, null, 2)}\n`);

function requiredArgument(name: string): string {
  const prefix = `${name}=`;
  const value = process.argv.slice(2).find((argument) => argument.startsWith(prefix))?.slice(prefix.length).trim();
  if (!value) throw new Error(`${name} is required`);
  return value;
}
