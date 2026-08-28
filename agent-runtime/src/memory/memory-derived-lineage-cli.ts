import { readFile, stat } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { createPool } from "mysql2/promise";

import { MySQLMemoryDerivedLineageStore, parseMemoryDerivedLineageManifest, type MemoryDerivedLineageStore } from "./memory-derived-lineage.js";

interface Writable { write(value: string): unknown }
interface StoreHandle { readonly store: MemoryDerivedLineageStore; close(): Promise<void> }
interface Dependencies { openStore(): StoreHandle }

export async function runMemoryDerivedLineageCLI(args: string[], stdout: Writable, stderr: Writable, dependencies: Dependencies = defaults()): Promise<number> {
  const matching = args.filter(argument => argument.startsWith("--manifest="));
  const path = matching[0]?.slice("--manifest=".length).trim();
  if (args.length !== 1 || matching.length !== 1 || !path) {
    stderr.write("Memory derived-lineage audit requires exactly one --manifest=<path> argument\n");
    return 1;
  }
  let handle: StoreHandle | undefined;
  try {
    const metadata = await stat(path);
    if (!metadata.isFile() || metadata.size > 16 * 1024) throw new Error("invalid manifest");
    const manifest = parseMemoryDerivedLineageManifest(await readFile(path, "utf8"));
    handle = dependencies.openStore();
    stdout.write(`${JSON.stringify(await handle.store.load(manifest), null, 2)}\n`);
    return 0;
  } catch {
    stderr.write("Memory derived-lineage audit failed closed\n");
    return 1;
  } finally {
    await handle?.close().catch(() => undefined);
  }
}

function defaults(): Dependencies {
  return { openStore: () => {
    const uri = process.env.DIPOLE_AGENT_EVAL_MYSQL_URL?.trim();
    if (!uri) throw new Error("missing read-only database");
    const pool = createPool({ uri, timezone: "Z", connectionLimit: 2 });
    return { store: new MySQLMemoryDerivedLineageStore(pool), close: () => pool.end() };
  } };
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runMemoryDerivedLineageCLI(process.argv.slice(2), process.stdout, process.stderr);
}
