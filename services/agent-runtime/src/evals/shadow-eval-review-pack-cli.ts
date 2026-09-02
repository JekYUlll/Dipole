import { pathToFileURL } from "node:url";

import { createPool } from "mysql2/promise";

import { MySQLShadowEvalObservationStore, type ShadowEvalObservationStore } from "./mysql-shadow-eval-store.js";
import { buildShadowEvalReviewPack } from "./shadow-eval-review-pack.js";

interface Writable {
  write(value: string): unknown;
}

interface StoreHandle {
  readonly store: ShadowEvalObservationStore;
  close(): Promise<void>;
}

interface ShadowEvalReviewPackCLIDependencies {
  openStore(): StoreHandle;
}

export async function runShadowEvalReviewPackCLI(
  args: readonly string[],
  stdout: Writable,
  stderr: Writable,
  dependencies: ShadowEvalReviewPackCLIDependencies = defaultDependencies()
): Promise<number> {
  const values = namedArguments(args, ["candidate-version", "task-id", "run-id"]);
  if (values === undefined) {
    stderr.write("shadow eval review pack requires exactly --candidate-version=<version> --task-id=<id> --run-id=<id> arguments\n");
    return 1;
  }

  let handle: StoreHandle | undefined;
  try {
    handle = dependencies.openStore();
    const observation = await handle.store.load(values["task-id"]!, values["run-id"]!);
    stdout.write(`${JSON.stringify(buildShadowEvalReviewPack(values["candidate-version"]!, observation), null, 2)}\n`);
    return 0;
  } catch (error) {
    stderr.write(`shadow eval review pack failed closed: ${error instanceof Error ? error.message : String(error)}\n`);
    return 1;
  } finally {
    await handle?.close().catch(() => undefined);
  }
}

function namedArguments(args: readonly string[], names: readonly string[]): Record<string, string> | undefined {
  if (args.length !== names.length) return undefined;
  const values: Record<string, string> = {};
  for (const argument of args) {
    const match = /^--([a-z-]+)=(.+)$/.exec(argument);
    if (match === null || !names.includes(match[1]!) || values[match[1]!] !== undefined || match[2]!.trim() === "") return undefined;
    values[match[1]!] = match[2]!.trim();
  }
  return names.every(name => values[name] !== undefined) ? values : undefined;
}

function defaultDependencies(): ShadowEvalReviewPackCLIDependencies {
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
  process.exitCode = await runShadowEvalReviewPackCLI(process.argv.slice(2), process.stdout, process.stderr);
}
