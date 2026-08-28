import { readFile, stat } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { evaluateOfflineEvalSuite } from "./offline-evaluator.js";
import {
  buildMemoryCorrectionEvalSuite,
  parseMemoryCorrectionEvalManifest,
  parseMemoryCorrectionEvalObservation
} from "./memory-correction-eval.js";

const maximumInputBytes = 64 * 1024;

interface Writable {
  write(value: string): unknown;
}

export async function runMemoryCorrectionEvalCLI(args: string[], stdout: Writable, stderr: Writable): Promise<number> {
  const manifestPath = singlePathArgument(args, "--manifest=");
  const observationPath = singlePathArgument(args, "--observation=");
  if (args.length !== 2 || manifestPath === undefined || observationPath === undefined) {
    stderr.write("Memory correction Eval requires exactly one --manifest=<path> and one --observation=<path> argument\n");
    return 1;
  }

  try {
    const [manifestSource, observationSource] = await Promise.all([
      readBoundedFile(manifestPath), readBoundedFile(observationPath)
    ]);
    const manifest = parseMemoryCorrectionEvalManifest(manifestSource);
    const observation = parseMemoryCorrectionEvalObservation(observationSource);
    const report = evaluateOfflineEvalSuite(buildMemoryCorrectionEvalSuite(manifest, observation));
    stdout.write(`${JSON.stringify(report, null, 2)}\n`);
    return report.passed ? 0 : 2;
  } catch {
    stderr.write("Memory correction Eval failed closed\n");
    return 1;
  }
}

function singlePathArgument(args: string[], prefix: string): string | undefined {
  const matching = args.filter(argument => argument.startsWith(prefix));
  if (matching.length !== 1) return undefined;
  const path = matching[0]!.slice(prefix.length).trim();
  return path === "" ? undefined : path;
}

async function readBoundedFile(path: string): Promise<string> {
  const metadata = await stat(path);
  if (!metadata.isFile() || metadata.size > maximumInputBytes) throw new Error("invalid input file");
  const source = await readFile(path, "utf8");
  if (Buffer.byteLength(source, "utf8") > maximumInputBytes) throw new Error("invalid input file");
  return source;
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runMemoryCorrectionEvalCLI(process.argv.slice(2), process.stdout, process.stderr);
}
