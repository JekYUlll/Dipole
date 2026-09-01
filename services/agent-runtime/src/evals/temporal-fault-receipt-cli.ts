import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { createTemporalFaultReceipt, validateTemporalFaultReceipt } from "./temporal-fault-receipt.js";

interface Writable { write(value: string): unknown; }

export async function runTemporalFaultReceiptCLI(args: string[], stdout: Writable, stderr: Writable): Promise<number> {
  const observations = args.filter(value => value.startsWith("--observation="));
  const receipts = args.filter(value => value.startsWith("--receipt="));
  if (
    args.length !== 1 || observations.length + receipts.length !== 1 ||
    (observations[0] !== undefined && observations[0].slice("--observation=".length).trim() === "") ||
    (receipts[0] !== undefined && receipts[0].slice("--receipt=".length).trim() === "")
  ) {
    stderr.write("temporal fault receipt requires exactly one --observation=<path> or --receipt=<path> argument\n");
    return 1;
  }
  try {
    const observation = observations[0];
    const source = observation ?? receipts[0]!;
    const raw = JSON.parse(await readFile(source.slice(source.indexOf("=") + 1), "utf8"));
    const receipt = observation === undefined ? validateTemporalFaultReceipt(raw) : createTemporalFaultReceipt(raw);
    stdout.write(`${JSON.stringify(receipt, null, 2)}\n`);
    return receipt.outcome === "eligible" ? 0 : 2;
  } catch (error) {
    stderr.write(`temporal fault receipt is invalid: ${error instanceof Error ? error.message : String(error)}\n`);
    return 1;
  }
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runTemporalFaultReceiptCLI(process.argv.slice(2), process.stdout, process.stderr);
}
