import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { collectSubscriptionShadowEvidenceInputFromPrometheus } from "./subscription-shadow-collector.js";

interface Writable { write(value: string): unknown; }
type Collector = (request: unknown) => Promise<unknown>;

export async function runSubscriptionShadowCollectorCLI(
  args: string[], stdout: Writable, stderr: Writable,
  collect: Collector = collectSubscriptionShadowEvidenceInputFromPrometheus
): Promise<number> {
  if (args.length !== 1 || !args[0]?.startsWith("--request=")) {
    stderr.write("subscription Shadow collector requires exactly one --request=<path> argument\n");
    return 1;
  }
  const path = args[0].slice(args[0].indexOf("=") + 1).trim();
  if (!path) {
    stderr.write("subscription Shadow collector argument is invalid\n");
    return 1;
  }
  try {
    const request: unknown = JSON.parse(await readFile(path, "utf8"));
    stdout.write(`${JSON.stringify(await collect(request), null, 2)}\n`);
    return 0;
  } catch {
    stderr.write("subscription Shadow collection failed\n");
    return 1;
  }
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runSubscriptionShadowCollectorCLI(process.argv.slice(2), process.stdout, process.stderr);
}
