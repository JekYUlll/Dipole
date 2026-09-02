import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { parseEventLeaseReclaimEvidence } from "./event-lease-reclaim-evidence.js";

interface Writable {
  write(value: string): unknown;
}

export async function runEventLeaseReclaimEvidenceCLI(
  args: string[],
  stdout: Writable,
  stderr: Writable,
  now: () => Date = () => new Date()
): Promise<number> {
  const evidenceArgs = args.filter(argument => argument.startsWith("--evidence="));
  if (args.length !== 1 || evidenceArgs.length !== 1 || evidenceArgs[0]!.slice("--evidence=".length).trim() === "") {
    stderr.write("Event lease reclaim evidence check requires exactly one --evidence=<path> argument\n");
    return 1;
  }
  try {
    const value: unknown = JSON.parse(await readFile(evidenceArgs[0]!.slice("--evidence=".length), "utf8"));
    stdout.write(`${JSON.stringify(parseEventLeaseReclaimEvidence(value, { now }), null, 2)}\n`);
    return 0;
  } catch {
    stderr.write("Event lease reclaim evidence is invalid\n");
    return 1;
  }
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runEventLeaseReclaimEvidenceCLI(process.argv.slice(2), process.stdout, process.stderr);
}
