import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { parseShadowEvalSummaryInput, summarizeShadowEvalReports } from "./shadow-eval-summary.js";

interface Writable {
  write(value: string): unknown;
}

const maximumInputBytes = 2 * 1024 * 1024;

export async function runShadowEvalSummaryCLI(args: readonly string[], stdout: Writable, stderr: Writable): Promise<number> {
  const inputArgs = args.filter(argument => argument.startsWith("--input="));
  if (args.length !== 1 || inputArgs.length !== 1 || inputArgs[0]!.slice("--input=".length).trim() === "") {
    stderr.write("shadow eval summary requires exactly one --input=<path> argument\n");
    return 1;
  }

  try {
    const source = await readFile(inputArgs[0]!.slice("--input=".length), "utf8");
    if (Buffer.byteLength(source, "utf8") > maximumInputBytes) throw new Error("Shadow evaluation summary input exceeds 2 MiB");
    const report = summarizeShadowEvalReports(parseShadowEvalSummaryInput(source));
    stdout.write(`${JSON.stringify(report, null, 2)}\n`);
    return report.summary.failedTasks === 0 ? 0 : 2;
  } catch (error) {
    stderr.write(`shadow eval summary failed closed: ${error instanceof Error ? error.message : String(error)}\n`);
    return 1;
  }
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runShadowEvalSummaryCLI(process.argv.slice(2), process.stdout, process.stderr);
}
