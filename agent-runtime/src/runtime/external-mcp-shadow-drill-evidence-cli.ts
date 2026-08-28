import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { parseExternalMcpShadowDrillEvidence } from "./external-mcp-shadow-drill-evidence.js";

interface Writable {
  write(value: string): unknown;
}

export async function runExternalMcpShadowDrillEvidenceCLI(
  args: string[],
  stdout: Writable,
  stderr: Writable,
  now: () => Date = () => new Date()
): Promise<number> {
  const evidenceArgs = args.filter(argument => argument.startsWith("--evidence="));
  if (args.length !== 1 || evidenceArgs.length !== 1 || evidenceArgs[0]!.slice("--evidence=".length).trim() === "") {
    stderr.write("external MCP Shadow drill check requires exactly one --evidence=<path> argument\n");
    return 1;
  }
  try {
    const value: unknown = JSON.parse(await readFile(evidenceArgs[0]!.slice("--evidence=".length), "utf8"));
    const evidence = parseExternalMcpShadowDrillEvidence(value, { now });
    stdout.write(`${JSON.stringify(evidence, null, 2)}\n`);
    return 0;
  } catch {
    stderr.write("external MCP Shadow drill evidence is invalid\n");
    return 1;
  }
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runExternalMcpShadowDrillEvidenceCLI(process.argv.slice(2), process.stdout, process.stderr);
}
