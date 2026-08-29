import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

import { createSubscriptionShadowEvidence, parseSubscriptionShadowEvidence } from "./subscription-shadow-evidence.js";

interface Writable { write(value: string): unknown; }

export async function runSubscriptionShadowEvidenceCLI(
  args: string[], stdout: Writable, stderr: Writable, now: () => Date = () => new Date()
): Promise<number> {
  const inputs = args.filter(value => value.startsWith("--input="));
  const evidenceArgs = args.filter(value => value.startsWith("--evidence="));
  if (args.length !== 1 || (inputs.length === 1) === (evidenceArgs.length === 1)) {
    stderr.write("subscription Shadow evidence requires exactly one --input=<path> or --evidence=<path> argument\n");
    return 1;
  }
  const argument = (inputs[0] ?? evidenceArgs[0])!;
  const rawPath = argument.slice(argument.indexOf("=") + 1).trim();
  if (!rawPath) {
    stderr.write("subscription Shadow evidence argument is invalid\n");
    return 1;
  }
  try {
    const value: unknown = JSON.parse(await readFile(rawPath, "utf8"));
    if (inputs.length === 1) {
      stdout.write(`${JSON.stringify(createSubscriptionShadowEvidence(value, { now }), null, 2)}\n`);
    } else {
      const evidence = parseSubscriptionShadowEvidence(value, { now });
      stdout.write(`${JSON.stringify({
        schema_version: evidence.schema_version, outcome: evidence.outcome, observed_events: evidence.observed_events,
        content_sha256: evidence.content_sha256, expires_at: evidence.expires_at,
        production_authority: false, runtime_change_authority: false
      })}\n`);
    }
    return 0;
  } catch {
    stderr.write("subscription Shadow evidence is invalid\n");
    return 1;
  }
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runSubscriptionShadowEvidenceCLI(process.argv.slice(2), process.stdout, process.stderr);
}
