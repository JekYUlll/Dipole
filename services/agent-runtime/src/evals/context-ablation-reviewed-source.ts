import { parseContextAblationManifest, type ContextAblationManifest } from "./context-ablation-adapter.js";
import { evaluateMemoryReviewedCorpus } from "./memory-reviewed-corpus.js";
import type { LoadedMemoryReviewedCorpus } from "./memory-reviewed-corpus-source.js";

export interface ContextAblationReviewedSourceReceipt {
  readonly sourceId: string;
  readonly corpusSha256: string;
  readonly reviewSha256: string;
}

/**
 * Binds an experiment to every approved corpus item without returning corpus
 * paths, case identifiers, reviewer identities, or source content.
 */
export function validateContextAblationReviewedSource(
  rawManifest: ContextAblationManifest | unknown,
  source: Pick<LoadedMemoryReviewedCorpus, "manifest" | "corpus" | "review">
): ContextAblationReviewedSourceReceipt {
  const manifest = parseContextAblationManifest(rawManifest);
  if (!evaluateMemoryReviewedCorpus(source.corpus, source.review).passed) {
    throw new Error("Context ablation reviewed corpus does not pass review gates");
  }
  const sourceCases = source.corpus.cases.map(item => item.contentSha256).sort();
  const experimentCases = manifest.cases.map(item => item.caseSha256).sort();
  if (sourceCases.length !== experimentCases.length || sourceCases.some((value, index) => value !== experimentCases[index])) {
    throw new Error("Context ablation cases must cover exactly the reviewed corpus content hashes");
  }
  return {
    sourceId: source.manifest.sourceId,
    corpusSha256: source.manifest.corpusSha256,
    reviewSha256: source.manifest.reviewSha256
  };
}
