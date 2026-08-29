import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { parseMemoryPrefilterEvidence, parseMemoryPrefilterPolicy } from "./memory-prefilter-evaluator.js";
import { evaluateMemoryPrefilterRollout } from "./memory-prefilter-rollout.js";
import { parseMemoryReviewedCorpus, parseMemoryReviewedCorpusReview } from "./memory-reviewed-corpus.js";

interface Writable {
  write(value: string): unknown;
}

export async function runMemoryPrefilterRolloutCLI(args: readonly string[], stdout: Writable = process.stdout, stderr: Writable = process.stderr): Promise<number> {
  const paths = Object.fromEntries(args.filter(item => item.startsWith("--")).map(item => { const [key, ...value] = item.slice(2).split("="); return [key, value.join("=")]; }));
  if (args.length !== 4 || paths.corpus === undefined || paths.review === undefined || paths.evidence === undefined || paths.policy === undefined) {
    stderr.write("memory prefilter rollout requires --corpus=<path> --review=<path> --evidence=<path> and --policy=<path>\n");
    return 1;
  }
  try {
    const [corpus, review, evidence, policy] = await Promise.all([paths.corpus, paths.review, paths.evidence, paths.policy].map(path => readFile(path, "utf8")));
    const decision = evaluateMemoryPrefilterRollout(parseMemoryReviewedCorpus(corpus), parseMemoryReviewedCorpusReview(review), parseMemoryPrefilterEvidence(evidence), parseMemoryPrefilterPolicy(policy));
    stdout.write(`${JSON.stringify(decision, null, 2)}\n`);
    return decision.decision === "eligible" ? 0 : 2;
  } catch (error) {
    stderr.write(`memory prefilter rollout input is invalid: ${error instanceof Error ? error.message : String(error)}\n`);
    return 1;
  }
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) process.exitCode = await runMemoryPrefilterRolloutCLI(process.argv.slice(2), process.stdout, process.stderr);
