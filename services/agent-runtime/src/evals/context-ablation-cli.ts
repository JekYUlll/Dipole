import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { createPool } from "mysql2/promise";

import { buildContextAblationEvalSuite, parseContextAblationManifest } from "./context-ablation-adapter.js";
import { evaluateContextAblation } from "./context-ablation-eval.js";
import { validateContextAblationReviewedSource } from "./context-ablation-reviewed-source.js";
import {
  loadMemoryReviewedCorpusSource,
  loadMemoryReviewedCorpusSourceManifest,
  type LoadedMemoryReviewedCorpus
} from "./memory-reviewed-corpus-source.js";
import { MySQLContextAblationObservationStore, type ContextAblationObservationStore } from "./mysql-context-ablation-store.js";
import { MySQLShadowEvalObservationStore } from "./mysql-shadow-eval-store.js";

interface Writable { write(value: string): unknown; }
interface StoreHandle { readonly store: ContextAblationObservationStore; close(): Promise<void>; }
interface ContextAblationCLIDependencies {
  openStore(): StoreHandle;
  loadReviewedSource?(path: string): Promise<LoadedMemoryReviewedCorpus>;
}

export async function runContextAblationCLI(
  args: readonly string[],
  stdout: Writable,
  stderr: Writable,
  dependencies: ContextAblationCLIDependencies = defaultDependencies()
): Promise<number> {
  const inputs = parseInputs(args);
  if (inputs === undefined) {
    stderr.write("context ablation requires --manifest=<path> and accepts one optional --reviewed-source=<path> argument\n");
    return 1;
  }

  let handle: StoreHandle | undefined;
  try {
    const manifest = parseContextAblationManifest(await readFile(inputs.manifestPath, "utf8"));
    const reviewedSource = inputs.reviewedSourcePath === undefined
      ? undefined
      : validateContextAblationReviewedSource(manifest, await (dependencies.loadReviewedSource ?? defaultLoadReviewedSource)(inputs.reviewedSourcePath));
    handle = dependencies.openStore();
    const observations = await handle.store.load(manifest.experimentId);
    const report = evaluateContextAblation(buildContextAblationEvalSuite(manifest, observations));
    stdout.write(`${JSON.stringify({ ...report, ...(reviewedSource === undefined ? {} : { reviewedSource }) }, null, 2)}\n`);
    return 0;
  } catch (error) {
    stderr.write(`context ablation failed closed: ${error instanceof Error ? error.message : String(error)}\n`);
    return 1;
  } finally {
    await handle?.close().catch(() => undefined);
  }
}

function parseInputs(args: readonly string[]): { manifestPath: string; reviewedSourcePath?: string } | undefined {
  if (args.length < 1 || args.length > 2) return undefined;
  const values = new Map<string, string>();
  for (const argument of args) {
    const match = /^--(manifest|reviewed-source)=(.+)$/u.exec(argument);
    if (match === null || values.has(match[1]!) || match[2]!.trim() === "") return undefined;
    values.set(match[1]!, match[2]!.trim());
  }
  const manifestPath = values.get("manifest");
  if (manifestPath === undefined) return undefined;
  const reviewedSourcePath = values.get("reviewed-source");
  return reviewedSourcePath === undefined ? { manifestPath } : { manifestPath, reviewedSourcePath };
}

async function defaultLoadReviewedSource(path: string): Promise<LoadedMemoryReviewedCorpus> {
  return loadMemoryReviewedCorpusSource(await loadMemoryReviewedCorpusSourceManifest(path));
}

function defaultDependencies(): ContextAblationCLIDependencies {
  return {
    openStore: () => {
      const uri = process.env.DIPOLE_AGENT_EVAL_MYSQL_URL?.trim();
      if (!uri) throw new Error("DIPOLE_AGENT_EVAL_MYSQL_URL is required for the read-only evaluation account");
      const pool = createPool({ uri, timezone: "Z", connectionLimit: 2 });
      const observations = new MySQLShadowEvalObservationStore(pool);
      return { store: new MySQLContextAblationObservationStore(pool, observations), close: () => pool.end() };
    }
  };
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runContextAblationCLI(process.argv.slice(2), process.stdout, process.stderr);
}
