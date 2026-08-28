import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { evaluateOfflineEvalSuite, parseOfflineEvalSuite } from "./offline-evaluator.js";

interface Writable {
  write(value: string): unknown;
}

export async function runOfflineEvalCLI(args: string[], stdout: Writable, stderr: Writable): Promise<number> {
  const suiteArgs = args.filter(argument => argument.startsWith("--suite="));
  if (args.length !== 1 || suiteArgs.length !== 1 || suiteArgs[0]!.slice("--suite=".length).trim() === "") {
    stderr.write("offline eval requires exactly one --suite=<path> argument\n");
    return 1;
  }

  try {
    const source = await readFile(suiteArgs[0]!.slice("--suite=".length), "utf8");
    const report = evaluateOfflineEvalSuite(parseOfflineEvalSuite(source));
    stdout.write(`${JSON.stringify(report, null, 2)}\n`);
    return report.passed ? 0 : 2;
  } catch (error) {
    stderr.write(`offline eval input is invalid: ${error instanceof Error ? error.message : String(error)}\n`);
    return 1;
  }
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runOfflineEvalCLI(process.argv.slice(2), process.stdout, process.stderr);
}
