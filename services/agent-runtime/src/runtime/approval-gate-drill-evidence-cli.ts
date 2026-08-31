import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { parseApprovalGateDrillEvidence } from "./approval-gate-drill-evidence.js";

interface Writable { write(value: string): unknown; }

export async function runApprovalGateDrillEvidenceCLI(args: string[], stdout: Writable, stderr: Writable, now: () => Date = () => new Date()): Promise<number> {
  const evidenceArgs = args.filter(argument => argument.startsWith("--evidence="));
  if (args.length !== 1 || evidenceArgs.length !== 1 || evidenceArgs[0]!.slice("--evidence=".length).trim() === "") {
    stderr.write("Approval gate drill check requires exactly one --evidence=<path> argument\n");
    return 1;
  }
  try {
    const evidence = parseApprovalGateDrillEvidence(JSON.parse(await readFile(evidenceArgs[0]!.slice("--evidence=".length), "utf8")), { now });
    stdout.write(`${JSON.stringify(evidence, null, 2)}\n`);
    return 0;
  } catch {
    stderr.write("Approval gate drill evidence is invalid\n");
    return 1;
  }
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runApprovalGateDrillEvidenceCLI(process.argv.slice(2), process.stdout, process.stderr);
}
