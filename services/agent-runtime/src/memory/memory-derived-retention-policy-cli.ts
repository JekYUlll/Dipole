import { readFile, stat } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { parseMemoryDerivedLineageReport } from "./memory-derived-lineage.js";
import {
  buildMemoryDerivedRetentionDecision,
  parseMemoryDerivedRetentionPolicy
} from "./memory-derived-retention-policy.js";

interface Writable { write(value: string): unknown }

export async function runMemoryDerivedRetentionPolicyCLI(
  args: string[],
  stdout: Writable,
  stderr: Writable
): Promise<number> {
  const policyPath = singlePath(args, "--policy=");
  const reportPath = singlePath(args, "--report=");
  if (args.length !== 2 || policyPath === undefined || reportPath === undefined) {
    stderr.write("Memory derived retention decision requires --policy=<path> and --report=<path>\n");
    return 1;
  }
  try {
    const [policyText, reportText] = await Promise.all([readBounded(policyPath), readBounded(reportPath)]);
    const decision = buildMemoryDerivedRetentionDecision(
      parseMemoryDerivedRetentionPolicy(policyText),
      parseMemoryDerivedLineageReport(reportText)
    );
    stdout.write(`${JSON.stringify(decision, null, 2)}\n`);
    return 0;
  } catch {
    stderr.write("Memory derived retention decision failed closed\n");
    return 1;
  }
}

function singlePath(args: string[], prefix: string): string | undefined {
  const matches = args.filter(argument => argument.startsWith(prefix));
  const path = matches[0]?.slice(prefix.length).trim();
  return matches.length === 1 && path ? path : undefined;
}

async function readBounded(path: string): Promise<string> {
  const metadata = await stat(path);
  if (!metadata.isFile() || metadata.size > 64 * 1024) throw new Error("invalid policy input");
  return readFile(path, "utf8");
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runMemoryDerivedRetentionPolicyCLI(process.argv.slice(2), process.stdout, process.stderr);
}
