import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { parseSubscriptionPrefilterCorpus } from "./subscription-prefilter-evaluator.js";
import { evaluateSubscriptionCorpusReview, parseSubscriptionCorpusReview } from "./subscription-corpus-review.js";

interface Writable {
  write(value: string): unknown;
}

export async function runSubscriptionCorpusReviewCLI(args: string[], stdout: Writable, stderr: Writable): Promise<number> {
  const corpusPath = exactArgument(args, "--corpus=");
  const reviewPath = exactArgument(args, "--review=");
  if (args.length !== 2 || corpusPath === undefined || reviewPath === undefined) {
    stderr.write("subscription corpus review requires --corpus=<path> and --review=<path>\n");
    return 1;
  }
  try {
    const corpus = parseSubscriptionPrefilterCorpus(await readFile(corpusPath, "utf8"));
    const review = parseSubscriptionCorpusReview(await readFile(reviewPath, "utf8"));
    const report = evaluateSubscriptionCorpusReview(corpus, review);
    stdout.write(`${JSON.stringify(report, null, 2)}\n`);
    return report.passed ? 0 : 2;
  } catch (error) {
    stderr.write(`subscription corpus review input is invalid: ${error instanceof Error ? error.message : String(error)}\n`);
    return 1;
  }
}

function exactArgument(args: string[], prefix: string): string | undefined {
  const matches = args.filter(argument => argument.startsWith(prefix));
  if (matches.length !== 1) return undefined;
  const value = matches[0]!.slice(prefix.length).trim();
  return value === "" ? undefined : value;
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runSubscriptionCorpusReviewCLI(process.argv.slice(2), process.stdout, process.stderr);
}
