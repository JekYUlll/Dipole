import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { evaluateMemoryPromotionWorkerDrill } from "./memory-promotion-worker-drill-evidence.js";

interface Writable {
  write(value: string): unknown;
}

export async function runMemoryPromotionWorkerDrillCLI(args: string[], stdout: Writable, stderr: Writable): Promise<number> {
  const evidenceArgs = args.filter(argument => argument.startsWith("--evidence="));
  if (args.length !== 1 || evidenceArgs.length !== 1 || evidenceArgs[0]!.slice("--evidence=".length).trim() === "") {
    stderr.write("memory promotion worker drill requires exactly one --evidence=<path> argument\n");
    return 1;
  }
  try {
    const evidence = JSON.parse(await readFile(evidenceArgs[0]!.slice("--evidence=".length), "utf8"));
    const decision = evaluateMemoryPromotionWorkerDrill(evidence);
    stdout.write(`${JSON.stringify(decision, null, 2)}\n`);
    return decision.decision === "eligible" ? 0 : 2;
  } catch (error) {
    stderr.write(`memory promotion worker drill evidence is invalid: ${error instanceof Error ? error.message : String(error)}\n`);
    return 1;
  }
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runMemoryPromotionWorkerDrillCLI(process.argv.slice(2), process.stdout, process.stderr);
}
