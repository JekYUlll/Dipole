import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { createPool } from "mysql2/promise";

import { evaluateOfflineEvalSuite } from "./offline-evaluator.js";
import { buildShadowEvalSuite, parseShadowEvalManifest } from "./shadow-eval-adapter.js";
import { createShadowEvalReport } from "./shadow-eval-report.js";
import { MySQLShadowEvalObservationStore, type ShadowEvalObservationStore } from "./mysql-shadow-eval-store.js";

interface Writable {
  write(value: string): unknown;
}

interface StoreHandle {
  readonly store: ShadowEvalObservationStore;
  close(): Promise<void>;
}

interface ShadowEvalCLIDependencies {
  openStore(): StoreHandle;
}

export async function runShadowEvalCLI(
  args: string[],
  stdout: Writable,
  stderr: Writable,
  dependencies: ShadowEvalCLIDependencies = defaultDependencies()
): Promise<number> {
  const manifestArgs = args.filter(argument => argument.startsWith("--manifest="));
  if (args.length !== 1 || manifestArgs.length !== 1 || manifestArgs[0]!.slice("--manifest=".length).trim() === "") {
    stderr.write("shadow eval requires exactly one --manifest=<path> argument\n");
    return 1;
  }

  let handle: StoreHandle | undefined;
  try {
    const source = await readFile(manifestArgs[0]!.slice("--manifest=".length), "utf8");
    const manifest = parseShadowEvalManifest(source);
    handle = dependencies.openStore();
    const observation = await handle.store.load(manifest.taskId, manifest.runId);
    const report = evaluateOfflineEvalSuite(buildShadowEvalSuite(manifest, observation));
    const shadowReport = createShadowEvalReport(observation.traceId, report);
    stdout.write(`${JSON.stringify(shadowReport, null, 2)}\n`);
    return report.passed ? 0 : 2;
  } catch (error) {
    stderr.write(`shadow eval failed closed: ${error instanceof Error ? error.message : String(error)}\n`);
    return 1;
  } finally {
    await handle?.close().catch(() => undefined);
  }
}

function defaultDependencies(): ShadowEvalCLIDependencies {
  return {
    openStore: () => {
      const uri = process.env.DIPOLE_AGENT_EVAL_MYSQL_URL?.trim();
      if (!uri) throw new Error("DIPOLE_AGENT_EVAL_MYSQL_URL is required for the read-only evaluation account");
      const pool = createPool({ uri, timezone: "Z", connectionLimit: 2 });
      return { store: new MySQLShadowEvalObservationStore(pool), close: () => pool.end() };
    }
  };
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runShadowEvalCLI(process.argv.slice(2), process.stdout, process.stderr);
}
