import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { parseAgentEventSubscription } from "../events/event-subscription.js";
import {
  evaluateSubscriptionPrefilter,
  parseSubscriptionPrefilterCorpus,
  parseSubscriptionPrefilterEvidence
} from "./subscription-prefilter-evaluator.js";
import { buildRulePrefilterEvidence } from "./subscription-prefilter-rule.js";

interface Writable {
  write(value: string): unknown;
}

export async function runSubscriptionPrefilterEvalCLI(args: string[], stdout: Writable, stderr: Writable): Promise<number> {
  const corpusPath = exactArgument(args, "--corpus=");
  const evidencePath = exactArgument(args, "--evidence=");
  const subscriptionPath = exactArgument(args, "--subscription=");
  if (args.length !== 2 || corpusPath === undefined || (evidencePath === undefined) === (subscriptionPath === undefined)) {
    stderr.write("subscription prefilter eval requires --corpus=<path> and exactly one of --evidence=<path> or --subscription=<path>\n");
    return 1;
  }
  try {
    const corpus = parseSubscriptionPrefilterCorpus(await readFile(corpusPath, "utf8"));
    if (subscriptionPath !== undefined) {
      const subscription = parseAgentEventSubscription(JSON.parse(await readFile(subscriptionPath, "utf8")) as unknown);
      const evidence = buildRulePrefilterEvidence(corpus, subscription);
      const report = evaluateSubscriptionPrefilter(corpus, evidence);
      stdout.write(`${JSON.stringify({ evidence, report }, null, 2)}\n`);
      return report.passed ? 0 : 2;
    }
    const evidence = parseSubscriptionPrefilterEvidence(await readFile(evidencePath!, "utf8"));
    const report = evaluateSubscriptionPrefilter(corpus, evidence);
    stdout.write(`${JSON.stringify(report, null, 2)}\n`);
    return report.passed ? 0 : 2;
  } catch (error) {
    stderr.write(`subscription prefilter eval input is invalid: ${error instanceof Error ? error.message : String(error)}\n`);
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
  process.exitCode = await runSubscriptionPrefilterEvalCLI(process.argv.slice(2), process.stdout, process.stderr);
}
