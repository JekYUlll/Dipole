import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { parseCoreRestartReadShadowEvidence } from "./core-restart-read-shadow-evidence.js";

interface Writable {
  write(value: string): unknown;
}

export async function runCoreRestartReadShadowEvidenceCLI(
  args: string[],
  stdout: Writable,
  stderr: Writable,
  now: () => Date = () => new Date()
): Promise<number> {
  const evidenceArgs = args.filter(argument => argument.startsWith("--evidence="));
  if (args.length !== 1 || evidenceArgs.length !== 1 || evidenceArgs[0]!.slice("--evidence=".length).trim() === "") {
    stderr.write("Core restart read-shadow evidence check requires exactly one --evidence=<path> argument\n");
    return 1;
  }
  try {
    const value: unknown = JSON.parse(await readFile(evidenceArgs[0]!.slice("--evidence=".length), "utf8"));
    stdout.write(`${JSON.stringify(parseCoreRestartReadShadowEvidence(value, { now }), null, 2)}\n`);
    return 0;
  } catch {
    stderr.write("Core restart read-shadow evidence is invalid\n");
    return 1;
  }
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runCoreRestartReadShadowEvidenceCLI(process.argv.slice(2), process.stdout, process.stderr);
}
