import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { createPool } from "mysql2/promise";

import { buildContextAblationBindingPlan } from "./context-ablation-binding-plan.js";
import { MySQLContextAblationBindingStore } from "./context-ablation-binding-store.js";
import { validateContextAblationReviewedSource } from "./context-ablation-reviewed-source.js";
import { loadMemoryReviewedCorpusSource, loadMemoryReviewedCorpusSourceManifest } from "./memory-reviewed-corpus-source.js";

interface Writable { write(value: string): unknown; }

/** Operator-only command. It is intentionally absent from the runtime startup path. */
export async function runContextAblationBindingCLI(args: readonly string[], stdout: Writable, stderr: Writable): Promise<number> {
  const inputs = parseInputs(args);
  if (inputs === undefined) {
    stderr.write("context ablation binding requires --manifest=<path>, --bindings=<path>, and --reviewed-source=<path>\\n");
    return 1;
  }
  const uri = process.env.DIPOLE_AGENT_EVAL_BIND_MYSQL_URL?.trim();
  if (!uri) {
    stderr.write("context ablation binding failed closed: DIPOLE_AGENT_EVAL_BIND_MYSQL_URL is required for the operator-only binding account\\n");
    return 1;
  }
  const pool = createPool({ uri, timezone: "Z", connectionLimit: 1 });
  try {
    const [manifest, bindings, reviewedSource] = await Promise.all([
      readFile(inputs.manifestPath, "utf8"),
      readFile(inputs.bindingsPath, "utf8"),
      loadMemoryReviewedCorpusSource(await loadMemoryReviewedCorpusSourceManifest(inputs.reviewedSourcePath))
    ]);
    const plan = buildContextAblationBindingPlan(manifest, bindings);
    const reviewed = validateContextAblationReviewedSource(manifest, reviewedSource);
    const receipt = await new MySQLContextAblationBindingStore(pool).apply(plan);
    stdout.write(`${JSON.stringify({ schemaVersion: "dipole.agent.context-ablation-binding-receipt.v1", experimentId: plan.experimentId, candidateVersion: plan.candidateVersion, reviewedSource: reviewed, ...receipt })}\\n`);
    return 0;
  } catch (error) {
    stderr.write(`context ablation binding failed closed: ${error instanceof Error ? error.message : String(error)}\\n`);
    return 1;
  } finally {
    await pool.end();
  }
}

function parseInputs(args: readonly string[]): { manifestPath: string; bindingsPath: string; reviewedSourcePath: string } | undefined {
  if (args.length !== 3) return undefined;
  const values = new Map<string, string>();
  for (const value of args) {
    const match = /^--(manifest|bindings|reviewed-source)=(.+)$/u.exec(value);
    if (match === null || values.has(match[1]!) || match[2]!.trim() === "") return undefined;
    values.set(match[1]!, match[2]!.trim());
  }
  const manifestPath = values.get("manifest");
  const bindingsPath = values.get("bindings");
  const reviewedSourcePath = values.get("reviewed-source");
  return manifestPath === undefined || bindingsPath === undefined || reviewedSourcePath === undefined
    ? undefined
    : { manifestPath, bindingsPath, reviewedSourcePath };
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runContextAblationBindingCLI(process.argv.slice(2), process.stdout, process.stderr);
}
