import { readFile } from "node:fs/promises";

import { evaluateCalibrationEvidence, parseCalibrationEvidenceJSON } from "./token-calibration-evidence.js";

export async function runTokenCalibrationCLI(
  argv: readonly string[],
  output: Pick<NodeJS.WriteStream, "write"> = process.stdout,
  errorOutput: Pick<NodeJS.WriteStream, "write"> = process.stderr
): Promise<number> {
  try {
    const evidencePath = requiredArgument(argv, "--evidence");
    const evidence = parseCalibrationEvidenceJSON(await readFile(evidencePath, "utf8"));
    const report = evaluateCalibrationEvidence(evidence);
    output.write(`${JSON.stringify(report, null, 2)}\n`);
    return report.eligible ? 0 : 2;
  } catch (error) {
    errorOutput.write(`${error instanceof Error ? error.message : String(error)}\n`);
    return 1;
  }
}

function requiredArgument(argv: readonly string[], name: string): string {
  const prefix = `${name}=`;
  const matches = argv.filter((argument) => argument.startsWith(prefix));
  if (argv.length !== 1 || matches.length !== 1 || matches[0]!.slice(prefix.length).trim() === "") {
    throw new Error(`exactly one ${name}=<path> argument is required`);
  }
  return matches[0]!.slice(prefix.length);
}

if (import.meta.url === `file://${process.argv[1]}`) {
  process.exitCode = await runTokenCalibrationCLI(process.argv.slice(2));
}
