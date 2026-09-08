import { realpath, stat, writeFile } from "node:fs/promises";
import { dirname, isAbsolute, resolve } from "node:path";
import { pathToFileURL } from "node:url";

import { createMemoryReviewedCorpusSourceManifest } from "./memory-reviewed-corpus-source.js";

interface Writable {
  write(value: string): unknown;
}

const argumentsRequired = ["source-id", "corpus", "review", "approved-at", "expires-at", "output"] as const;

export async function runMemoryReviewedCorpusManifestCLI(args: readonly string[], stdout: Writable, stderr: Writable): Promise<number> {
  const values = parseArguments(args);
  if (values === undefined) {
    stderr.write("memory corpus manifest requires --source-id, --corpus, --review, --approved-at, --expires-at, and --output\n");
    return 1;
  }
  try {
    const outputPath = await newOutputPath(values.output);
    const manifest = await createMemoryReviewedCorpusSourceManifest({
      sourceId: values["source-id"], corpusPath: values.corpus, reviewPath: values.review,
      approvedAt: values["approved-at"], expiresAt: values["expires-at"]
    });
    await writeFile(outputPath, `${JSON.stringify(manifest, null, 2)}\n`, { encoding: "utf8", flag: "wx", mode: 0o600 });
    stdout.write(`${JSON.stringify({ schemaVersion: "dipole.agent.memory-reviewed-corpus-manifest-receipt.v1", sourceId: manifest.sourceId, corpusSha256: manifest.corpusSha256, reviewSha256: manifest.reviewSha256, expiresAt: manifest.expiresAt })}\n`);
    return 0;
  } catch (error) {
    stderr.write(`memory corpus manifest input is invalid: ${error instanceof Error ? error.message : String(error)}\n`);
    return 1;
  }
}

function parseArguments(args: readonly string[]): Record<(typeof argumentsRequired)[number], string> | undefined {
  if (args.length !== argumentsRequired.length) return undefined;
  const values = new Map<string, string>();
  for (const argument of args) {
    const match = /^--([a-z-]+)=(.+)$/u.exec(argument);
    const name = match?.[1];
    const value = match?.[2];
    if (name === undefined || value === undefined || !argumentsRequired.includes(name as (typeof argumentsRequired)[number]) || values.has(name) || value.trim() === "") return undefined;
    values.set(name, value.trim());
  }
  if (values.size !== argumentsRequired.length) return undefined;
  return Object.fromEntries(argumentsRequired.map(name => [name, values.get(name)!])) as Record<(typeof argumentsRequired)[number], string>;
}

async function newOutputPath(rawPath: string): Promise<string> {
  if (!isAbsolute(rawPath)) throw new Error("memory corpus manifest output must be absolute");
  const outputPath = resolve(rawPath);
  if (outputPath !== rawPath) throw new Error("memory corpus manifest output must be canonical");
  const parent = dirname(outputPath);
  if (await realpath(parent) !== parent || !(await stat(parent)).isDirectory()) throw new Error("memory corpus manifest output parent is invalid");
  return outputPath;
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await runMemoryReviewedCorpusManifestCLI(process.argv.slice(2), process.stdout, process.stderr);
}
