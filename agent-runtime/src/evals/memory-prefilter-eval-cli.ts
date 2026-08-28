import { readFile } from "node:fs/promises";

import { evaluateMemoryPrefilter, parseMemoryPrefilterEvidence, parseMemoryPrefilterPolicy } from "./memory-prefilter-evaluator.js";
import { parseMemoryReviewedCorpus } from "./memory-reviewed-corpus.js";

export async function runMemoryPrefilterEvalCLI(args: readonly string[], stdout = process.stdout, stderr = process.stderr): Promise<number> {
  const options = Object.fromEntries(args.filter(arg => arg.startsWith("--")).map(arg => {
    const [key, ...value] = arg.slice(2).split("=");
    return [key, value.join("=")];
  }));
  if (options.corpus === undefined || options.evidence === undefined || options.policy === undefined) {
    stderr.write("memory prefilter eval requires --corpus=<path> --evidence=<path> and --policy=<path>\n");
    return 1;
  }
  try {
    const [corpus, evidence, policy] = await Promise.all([
      readFile(options.corpus, "utf8"), readFile(options.evidence, "utf8"), readFile(options.policy, "utf8")
    ]);
    const report = evaluateMemoryPrefilter(parseMemoryReviewedCorpus(corpus), parseMemoryPrefilterEvidence(evidence), parseMemoryPrefilterPolicy(policy));
    stdout.write(`${JSON.stringify(report)}\n`);
    return report.passed ? 0 : 2;
  } catch (error) {
    stderr.write(`memory prefilter eval input is invalid: ${error instanceof Error ? error.message : String(error)}\n`);
    return 1;
  }
}

if (import.meta.url === `file://${process.argv[1]}`) process.exitCode = await runMemoryPrefilterEvalCLI(process.argv.slice(2));
