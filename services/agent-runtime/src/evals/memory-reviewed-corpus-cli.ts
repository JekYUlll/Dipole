import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { evaluateMemoryReviewedCorpus, parseMemoryReviewedCorpus, parseMemoryReviewedCorpusReview } from "./memory-reviewed-corpus.js";
import { loadMemoryReviewedCorpusSource } from "./memory-reviewed-corpus-source.js";

interface Writable {
  write(value: string): unknown;
}

export async function runMemoryReviewedCorpusCLI(args: readonly string[], stdout: Writable, stderr: Writable): Promise<number> {
  const corpusPath = argument(args, "corpus");
  const reviewPath = argument(args, "review");
  const manifestPath = argument(args, "manifest");
  if (manifestPath === undefined && (corpusPath === undefined || reviewPath === undefined)) {
    stderr.write("memory corpus review requires --manifest=<path> or --corpus=<path> and --review=<path>\n");
    return 1;
  }
  try {
    const loaded = manifestPath === undefined
      ? undefined
      : await loadMemoryReviewedCorpusSource(JSON.parse(await readFile(manifestPath, "utf8")) as unknown);
    const corpus = loaded?.corpus ?? parseMemoryReviewedCorpus(await readFile(corpusPath!, "utf8"));
    const review = loaded?.review ?? parseMemoryReviewedCorpusReview(await readFile(reviewPath!, "utf8"));
    const report = evaluateMemoryReviewedCorpus(corpus, review);
    stdout.write(`${JSON.stringify(report)}\n`);
    return report.passed ? 0 : 2;
  } catch (error) {
    stderr.write(`memory corpus review input is invalid: ${error instanceof Error ? error.message : String(error)}\n`);
    return 1;
  }
}

function argument(args: readonly string[], name: string): string | undefined {
  const prefix = `--${name}=`;
  const value = args.find(item => item.startsWith(prefix))?.slice(prefix.length).trim();
  return value === "" ? undefined : value;
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runMemoryReviewedCorpusCLI(process.argv.slice(2), process.stdout, process.stderr);
}
