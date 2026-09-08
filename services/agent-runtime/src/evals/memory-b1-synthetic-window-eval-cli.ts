import { readFile, stat, writeFile } from "node:fs/promises";
import { dirname, isAbsolute } from "node:path";
import { pathToFileURL } from "node:url";

import { evaluateMemoryB1SyntheticWindow } from "./memory-b1-synthetic-window-eval.js";

const maximumInputBytes = 2 * 1024 * 1024;
interface Writable { write(value: string): unknown; }

export async function runMemoryB1SyntheticWindowCLI(args: readonly string[], stdout: Writable, stderr: Writable): Promise<number> {
  const windows = args.filter(argument => argument.startsWith("--window="));
  const reports = args.filter(argument => argument.startsWith("--report="));
  if (args.length !== windows.length + reports.length || windows.length !== 1 || reports.length > 1 || windows[0]!.slice(9).trim() === "" || (reports[0] !== undefined && reports[0].slice(9).trim() === "")) {
    stderr.write("B1 synthetic window requires exactly one --window=<path> and an optional --report=<absolute-new-path> argument\n");
    return 1;
  }
  try {
    const path = windows[0]!.slice(9).trim();
    const metadata = await stat(path);
    if (!metadata.isFile() || metadata.size > maximumInputBytes) throw new Error("invalid window file");
    const source = await readFile(path, "utf8");
    if (Buffer.byteLength(source, "utf8") > maximumInputBytes) throw new Error("invalid window file");
    const report = evaluateMemoryB1SyntheticWindow(source);
    const reportPath = reports[0]?.slice(9).trim();
    if (reportPath !== undefined) {
      if (!isAbsolute(reportPath)) throw new Error("invalid report path");
      const parent = await stat(dirname(reportPath));
      if (!parent.isDirectory()) throw new Error("invalid report path");
      await writeFile(reportPath, `${JSON.stringify(report, null, 2)}\n`, { encoding: "utf8", flag: "wx", mode: 0o600 });
    }
    stdout.write(`${JSON.stringify(report)}\n`);
    return report.passed ? 0 : 2;
  } catch {
    stderr.write("B1 synthetic window failed closed\n");
    return 1;
  }
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runMemoryB1SyntheticWindowCLI(process.argv.slice(2), process.stdout, process.stderr);
}
