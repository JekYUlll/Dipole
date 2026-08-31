import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { createTemporalFaultReceipt } from "./temporal-fault-receipt.js";

interface Writable { write(value: string): unknown; }

export async function runTemporalFaultReceiptCLI(args: string[], stdout: Writable, stderr: Writable): Promise<number> {
  const evidence = args.filter(value => value.startsWith("--observation="));
  if (args.length !== 1 || evidence.length !== 1 || evidence[0]!.slice("--observation=".length).trim() === "") {
    stderr.write("temporal fault receipt requires exactly one --observation=<path> argument\n");
    return 1;
  }
  try {
    const receipt = createTemporalFaultReceipt(JSON.parse(await readFile(evidence[0]!.slice("--observation=".length), "utf8")));
    stdout.write(`${JSON.stringify(receipt, null, 2)}\n`);
    return receipt.outcome === "eligible" ? 0 : 2;
  } catch (error) {
    stderr.write(`temporal fault receipt observation is invalid: ${error instanceof Error ? error.message : String(error)}\n`);
    return 1;
  }
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runTemporalFaultReceiptCLI(process.argv.slice(2), process.stdout, process.stderr);
}
