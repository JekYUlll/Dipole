import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { createPool } from "mysql2/promise";

import { buildContextAblationEvalSuite, parseContextAblationManifest } from "./context-ablation-adapter.js";
import { evaluateContextAblation } from "./context-ablation-eval.js";
import { MySQLContextAblationObservationStore, type ContextAblationObservationStore } from "./mysql-context-ablation-store.js";
import { MySQLShadowEvalObservationStore } from "./mysql-shadow-eval-store.js";

interface Writable { write(value: string): unknown; }
interface StoreHandle { readonly store: ContextAblationObservationStore; close(): Promise<void>; }
interface ContextAblationCLIDependencies { openStore(): StoreHandle; }

export async function runContextAblationCLI(
  args: readonly string[],
  stdout: Writable,
  stderr: Writable,
  dependencies: ContextAblationCLIDependencies = defaultDependencies()
): Promise<number> {
  const manifestArgs = args.filter(argument => argument.startsWith("--manifest="));
  if (args.length !== 1 || manifestArgs.length !== 1 || manifestArgs[0]!.slice("--manifest=".length).trim() === "") {
    stderr.write("context ablation requires exactly one --manifest=<path> argument\n");
    return 1;
  }

  let handle: StoreHandle | undefined;
  try {
    const manifest = parseContextAblationManifest(await readFile(manifestArgs[0]!.slice("--manifest=".length), "utf8"));
    handle = dependencies.openStore();
    const observations = await handle.store.load(manifest.experimentId);
    const report = evaluateContextAblation(buildContextAblationEvalSuite(manifest, observations));
    stdout.write(`${JSON.stringify(report, null, 2)}\n`);
    return 0;
  } catch (error) {
    stderr.write(`context ablation failed closed: ${error instanceof Error ? error.message : String(error)}\n`);
    return 1;
  } finally {
    await handle?.close().catch(() => undefined);
  }
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
