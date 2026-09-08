import { readFile, stat } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { evaluateMemoryB1SyntheticEval } from "./memory-b1-synthetic-eval.js";

const maximumInputBytes = 256 * 1024;
interface Writable { write(value: string): unknown; }

export async function runMemoryB1SyntheticEvalCLI(args: readonly string[], stdout: Writable, stderr: Writable): Promise<number> {
  const matching = args.filter(argument => argument.startsWith("--suite="));
  if (args.length !== 1 || matching.length !== 1 || matching[0]!.slice(8).trim() === "") {
    stderr.write("B1 synthetic Eval requires exactly one --suite=<path> argument\n");
    return 1;
  }
  try {
    const path = matching[0]!.slice(8).trim();
    const metadata = await stat(path);
    if (!metadata.isFile() || metadata.size > maximumInputBytes) throw new Error("invalid suite file");
    const source = await readFile(path, "utf8");
    if (Buffer.byteLength(source, "utf8") > maximumInputBytes) throw new Error("invalid suite file");
    const report = evaluateMemoryB1SyntheticEval(source);
    stdout.write(`${JSON.stringify(report)}\n`);
    return report.passed ? 0 : 2;
  } catch {
    stderr.write("B1 synthetic Eval failed closed\n");
    return 1;
  }
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runMemoryB1SyntheticEvalCLI(process.argv.slice(2), process.stdout, process.stderr);
}
