import { readFile, writeFile } from "node:fs/promises";

const sourceUrl = new URL("../db/queries/agent_shadow_trajectory.sql", import.meta.url);
const outputUrl = new URL("../services/agent-runtime/src/events/mysql-shadow-audit-queries.ts", import.meta.url);
const source = await readFile(sourceUrl, "utf8");
const entries = [...source.matchAll(/^-- name: (\w+) :\w+\n([\s\S]*?)(?=^-- name:|(?![\s\S]))/gm)];

if (entries.length !== 10) {
  throw new Error(`expected 10 Agent Shadow trajectory queries, found ${entries.length}`);
}

const output = [
  "// Code generated from db/queries/agent_shadow_trajectory.sql; DO NOT EDIT.",
  ""
];
for (const [, name, sql] of entries) {
  const constant = name.replaceAll(/([a-z0-9])([A-Z])/g, "$1_$2").toUpperCase();
  output.push(`export const ${constant} = ${JSON.stringify(sql.trim().replace(/;$/, ""))};`, "");
}
await writeFile(outputUrl, `${output.join("\n").trimEnd()}\n`);
