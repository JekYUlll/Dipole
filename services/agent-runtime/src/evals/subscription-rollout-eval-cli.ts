import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { parseSubscriptionCorpusReview } from "./subscription-corpus-review.js";
import { parseSubscriptionPrefilterCorpus, parseSubscriptionPrefilterEvidence } from "./subscription-prefilter-evaluator.js";
import { evaluateSubscriptionRollout } from "./subscription-rollout-evaluator.js";

interface Writable {
  write(value: string): unknown;
}

export async function runSubscriptionRolloutEvalCLI(args: string[], stdout: Writable, stderr: Writable): Promise<number> {
  const corpusPath = exactArgument(args, "--corpus=");
  const reviewPath = exactArgument(args, "--review=");
  const evidencePath = exactArgument(args, "--evidence=");
  if (args.length !== 3 || corpusPath === undefined || reviewPath === undefined || evidencePath === undefined) {
    stderr.write("subscription rollout eval requires --corpus=<path>, --review=<path>, and --evidence=<path>\n");
    return 1;
  }
  try {
    const [corpusSource, reviewSource, evidenceSource] = await Promise.all([
      readFile(corpusPath, "utf8"), readFile(reviewPath, "utf8"), readFile(evidencePath, "utf8")
    ]);
    const decision = evaluateSubscriptionRollout(
      parseSubscriptionPrefilterCorpus(corpusSource),
      parseSubscriptionCorpusReview(reviewSource),
      parseSubscriptionPrefilterEvidence(evidenceSource)
    );
    stdout.write(`${JSON.stringify(decision, null, 2)}\n`);
    return decision.decision === "eligible" ? 0 : 2;
  } catch (error) {
    stderr.write(`subscription rollout evidence is invalid: ${error instanceof Error ? error.message : String(error)}\n`);
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
  process.exitCode = await runSubscriptionRolloutEvalCLI(process.argv.slice(2), process.stdout, process.stderr);
}
